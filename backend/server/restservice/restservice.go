package restservice

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/go-openapi/runtime/flagext"
	"github.com/go-openapi/swag"
	"github.com/go-pg/pg/v10"
	flags "github.com/jessevdk/go-flags"
	pkgerrors "github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/netutil"

	"isc.org/stork/server/agentcomm"
	"isc.org/stork/server/apps"
	"isc.org/stork/server/configreview"
	dbops "isc.org/stork/server/database"
	dbsession "isc.org/stork/server/database/session"
	"isc.org/stork/server/eventcenter"
	"isc.org/stork/server/gen/restapi"
	"isc.org/stork/server/gen/restapi/operations"
	"isc.org/stork/server/metrics"
)

type RestAPISettings struct {
	CleanupTimeout  time.Duration    `long:"rest-cleanup-timeout" description:"grace period for which to wait before killing idle connections" default:"10s"`
	GracefulTimeout time.Duration    `long:"rest-graceful-timeout" description:"grace period for which to wait before shutting down the server" default:"15s"`
	MaxHeaderSize   flagext.ByteSize `long:"rest-max-header-size" description:"controls the maximum number of bytes the server will read parsing the request header's keys and values, including the request line. It does not limit the size of the request body." default:"1MiB"`

	Host         string        `long:"rest-host" description:"the IP to listen on" default:"" env:"STORK_REST_HOST"`
	Port         int           `long:"rest-port" description:"the port to listen on for connections" default:"8080" env:"STORK_REST_PORT"`
	ListenLimit  int           `long:"rest-listen-limit" description:"limit the number of outstanding requests"`
	KeepAlive    time.Duration `long:"rest-keep-alive" description:"set the TCP keep-alive timeouts on accepted connections. It prunes dead TCP connections ( e.g. closing laptop mid-download)" default:"3m"`
	ReadTimeout  time.Duration `long:"rest-read-timeout" description:"maximum duration before timing out read of the request" default:"30s"`
	WriteTimeout time.Duration `long:"rest-write-timeout" description:"maximum duration before timing out write of the response" default:"60s"`

	TLSCertificate    flags.Filename `long:"rest-tls-certificate" description:"the certificate to use for secure connections" env:"STORK_REST_TLS_CERTIFICATE"`
	TLSCertificateKey flags.Filename `long:"rest-tls-key" description:"the private key to use for secure connections" env:"STORK_REST_TLS_PRIVATE_KEY"`
	TLSCACertificate  flags.Filename `long:"rest-tls-ca" description:"the certificate authority file to be used with mutual tls auth" env:"STORK_REST_TLS_CA_CERTIFICATE"`

	StaticFilesDir string `long:"rest-static-files-dir" description:"Directory with static files for UI" default:"" env:"STORK_REST_STATIC_FILES_DIR"`
}

// Runtime information and settings for ReST API service.
type RestAPI struct {
	Settings         *RestAPISettings
	DBSettings       *dbops.DatabaseSettings
	DB               *dbops.PgDB
	SessionManager   *dbsession.SessionMgr
	EventCenter      eventcenter.EventCenter
	Pullers          *apps.Pullers
	ReviewDispatcher configreview.Dispatcher
	MetricsCollector metrics.Collector

	Agents agentcomm.ConnectedAgents

	TLS          bool
	HTTPServer   *http.Server
	srvListener  net.Listener
	api          *operations.StorkAPI
	handler      http.Handler
	hasListeners bool
	Host         string // actual host for listening
	Port         int    // actual port for listening
}

// Instantiates RestAPI structure.
//
// This function exposes flexible interface for passing pointers to the
// structures required by the RestAPI to operate. The RestAPI struct
// comprises several pointers and several interfaces initialized by a
// caller. They are specified as variadic function arguments in no
// particular order. The function will detect their types and assign
// them to appropriate RestAPI fields.
//
// Accepted pointers:
// - *RestAPISettings,
// - *dbops.DatabaseSettings,
// - *pg.DB,
// - *apps.Pullers,
//
// Accepted interfaces:
// - agentcomm.ConnectedAgents,
// - configreview.Dispatcher
// - eventcenter.EventCenter,
// - metrics.Collector
//
// The only mandatory parameter is the *dbops.DatabaseSettings because it
// is used to instantiate the Session Manager. Other parameters are
// optional but, if they are not specified, it may lead to nil pointer
// dereference issues upon using the RestAPI instance. Specifying a
// subset of the arguments is mostly useful in the unit tests which
// test specific RestAPI functionality.
//
// Upon adding new fields to the RestAPI, the function may be easily
// extended to support them.
func NewRestAPI(args ...interface{}) (*RestAPI, error) {
	api := &RestAPI{}

	// Iterate over the variadic arguments.
	for i, arg := range args {
		argType := reflect.TypeOf(arg)

		// If the interface is nil the TypeOf returns nil. Move to
		// the next argument.
		if argType == nil {
			continue
		}

		// Make sure that the specified argument is a pointer.
		if argType.Kind() != reflect.Ptr {
			return nil, pkgerrors.Errorf("non-pointer argument specified for NewRestAPI at position %d", i)
		}

		// If the value is nil, there is nothing to do. Move on.
		if reflect.ValueOf(arg).IsNil() {
			continue
		}

		// The underlying type must be a struct.
		if argType.Elem().Kind() != reflect.Struct {
			return nil, pkgerrors.Errorf("pointer to non-struct argument specified for NewRestAPI at position %d", i)
		}

		// Check if the specified argument is an interface.
		if argType.Implements(reflect.TypeOf((*agentcomm.ConnectedAgents)(nil)).Elem()) {
			api.Agents = arg.(agentcomm.ConnectedAgents)
			continue
		}
		if argType.Implements(reflect.TypeOf((*configreview.Dispatcher)(nil)).Elem()) {
			api.ReviewDispatcher = arg.(configreview.Dispatcher)
			continue
		}
		if argType.Implements(reflect.TypeOf((*eventcenter.EventCenter)(nil)).Elem()) {
			api.EventCenter = arg.(eventcenter.EventCenter)
			continue
		}
		if argType.Implements(reflect.TypeOf((*metrics.Collector)(nil)).Elem()) {
			api.MetricsCollector = arg.(metrics.Collector)
			continue
		}

		// Check if the specified argument is one of our supported structures.
		if argType.AssignableTo(reflect.TypeOf((*dbops.DatabaseSettings)(nil))) {
			api.DBSettings = arg.(*dbops.DatabaseSettings)
			continue
		}
		if argType.AssignableTo(reflect.TypeOf((*pg.DB)(nil))) {
			api.DB = arg.(*pg.DB)
			continue
		}
		if argType.AssignableTo(reflect.TypeOf((*apps.Pullers)(nil))) {
			api.Pullers = arg.(*apps.Pullers)
			continue
		}
		if argType.AssignableTo(reflect.TypeOf((*RestAPISettings)(nil))) {
			api.Settings = arg.(*RestAPISettings)
			continue
		}
		return nil, pkgerrors.Errorf("unknown argument type %s specified for NewRestAPI", argType.Elem().Name())
	}

	// Database settings must be specified because we need to instantiate the
	// session manager.
	if api.DBSettings == nil {
		return nil, pkgerrors.Errorf("dbops.DatabaseSettings parameter is required in NewRestAPI call")
	}

	// Instantiate the session manager.
	sm, err := dbsession.NewSessionMgr(&api.DBSettings.BaseDatabaseSettings)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "unable to establish connection to the session database")
	}
	api.SessionManager = sm

	// All ok.
	return api, nil
}

func prepareTLS(httpServer *http.Server, s *RestAPISettings) error {
	var err error

	// Inspired by https://blog.bracebin.com/achieving-perfect-ssl-labs-score-with-go
	httpServer.TLSConfig = &tls.Config{
		// Causes servers to use Go's default ciphersuite preferences,
		// which are tuned to avoid attacks. Does nothing on clients.
		PreferServerCipherSuites: true,
		// Only use curves which have assembly implementations
		// https://github.com/golang/go/tree/master/src/crypto/elliptic
		CurvePreferences: []tls.CurveID{tls.CurveP256},
		// Use modern tls mode https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility
		NextProtos: []string{"h2", "http/1.1"},
		// https://www.owasp.org/index.php/Transport_Layer_Protection_Cheat_Sheet#Rule_-_Only_Support_Strong_Protocols
		MinVersion: tls.VersionTLS12,
		// These ciphersuites support Forward Secrecy: https://en.wikipedia.org/wiki/Forward_secrecy
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}

	// build standard config from server options
	if s.TLSCertificate != "" && s.TLSCertificateKey != "" {
		httpServer.TLSConfig.Certificates = make([]tls.Certificate, 1)
		httpServer.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(string(s.TLSCertificate), string(s.TLSCertificateKey))
		if err != nil {
			return pkgerrors.Wrap(err, "problem with setting up certificates")
		}
	}

	if s.TLSCACertificate != "" {
		// include specified CA certificate
		caCert, caCertErr := os.ReadFile(string(s.TLSCACertificate))
		if caCertErr != nil {
			return pkgerrors.Wrap(caCertErr, "problem with setting up certificates")
		}
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(caCert)
		if !ok {
			return pkgerrors.New("cannot parse CA certificate")
		}
		httpServer.TLSConfig.ClientCAs = caCertPool
		httpServer.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if len(httpServer.TLSConfig.Certificates) == 0 && httpServer.TLSConfig.GetCertificate == nil {
		// after standard and custom config are passed, this ends up with no certificate
		if s.TLSCertificate == "" {
			if s.TLSCertificateKey == "" {
				log.Fatalf("the required flags `--tls-certificate` and `--tls-key` were not specified")
			}
			log.Fatalf("the required flag `--tls-certificate` was not specified")
		}
		if s.TLSCertificateKey == "" {
			log.Fatalf("the required flag `--tls-key` was not specified")
		}
		// this happens with a wrong custom TLS configurator
		log.Fatalf("no certificate was configured for TLS")
	}

	// must have at least one certificate or panics
	httpServer.TLSConfig.BuildNameToCertificate()

	return nil
}

// Serve the API.
func (r *RestAPI) Serve() (err error) {
	// Initiate the http handler, with the objects that are implementing the business logic.
	h, err := restapi.Handler(restapi.Config{
		GeneralAPI:      r,
		ServicesAPI:     r,
		UsersAPI:        r,
		DhcpAPI:         r,
		SettingsAPI:     r,
		SearchAPI:       r,
		EventsAPI:       r,
		Logger:          log.Infof,
		InnerMiddleware: r.InnerMiddleware,
		Authorizer:      r.Authorizer,
		AuthToken: func(token string) (interface{}, error) {
			// In normal circumstances we'd need to return some
			// user information here, but the authentication is
			// currently done in the middleware anyway, so we
			// bypass this whole mechanism anyway. Let's just
			// return the token.
			return token, nil
		},
	})
	if err != nil {
		return pkgerrors.Wrap(err, "cannot setup ReST API handler")
	}
	r.handler = h

	if !r.hasListeners {
		if err = r.Listen(); err != nil {
			return err
		}
	}

	// set default handler, if none is set
	if r.handler == nil {
		if r.api == nil {
			return pkgerrors.New("can't create the default handler, as no API is set")
		}

		r.handler = r.api.Serve(nil)
	}

	s := r.Settings

	httpServer := new(http.Server)
	r.HTTPServer = httpServer
	httpServer.MaxHeaderBytes = int(s.MaxHeaderSize)
	httpServer.ReadTimeout = s.ReadTimeout
	httpServer.WriteTimeout = s.WriteTimeout
	httpServer.SetKeepAlivesEnabled(int64(s.KeepAlive) > 0)
	if s.ListenLimit > 0 {
		r.srvListener = netutil.LimitListener(r.srvListener, s.ListenLimit)
	}
	if int64(s.CleanupTimeout) > 0 {
		httpServer.IdleTimeout = s.CleanupTimeout
	}

	if s.StaticFilesDir == "" {
		s.StaticFilesDir = "./webui/dist/stork"
	}
	httpServer.Handler = r.GlobalMiddleware(r.handler, s.StaticFilesDir, r.EventCenter)

	if r.TLS {
		err = prepareTLS(httpServer, s)
		if err != nil {
			return err
		}
	}

	var lstnr net.Listener
	var scheme string
	if !r.TLS {
		lstnr = r.srvListener
		scheme = "http://"
	} else {
		lstnr = tls.NewListener(r.srvListener, httpServer.TLSConfig)
		scheme = "https://"
	}

	log.WithFields(log.Fields{
		"address": scheme + lstnr.Addr().String(),
	}).Infof("started serving Stork Server")
	if err := httpServer.Serve(lstnr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return pkgerrors.Wrap(err, "problem with serving")
	}
	log.Info("stopped serving Stork Server")

	return nil
}

// Listen creates the listeners for the server.
func (r *RestAPI) Listen() error {
	if r.hasListeners { // already done this
		return nil
	}

	s := r.Settings

	if s.TLSCertificate == "" {
		r.TLS = false
	} else {
		r.TLS = true
	}

	if !r.TLS {
		// TLS disabled
		listener, err := net.Listen("tcp", net.JoinHostPort(s.Host, strconv.Itoa(s.Port)))
		if err != nil {
			return pkgerrors.Wrap(err, "problem occurred while starting to listen using ReST API")
		}

		h, p, err := swag.SplitHostPort(listener.Addr().String())
		if err != nil {
			return pkgerrors.Wrap(err, "problem with address")
		}
		r.Host = h
		r.Port = p
		r.srvListener = listener
	} else {
		// TLS enabled

		tlsListener, err := net.Listen("tcp", net.JoinHostPort(s.Host, strconv.Itoa(s.Port)))
		if err != nil {
			return pkgerrors.Wrap(err, "problem occurred while starting to listen using ReST API")
		}

		sh, sp, err := swag.SplitHostPort(tlsListener.Addr().String())
		if err != nil {
			return pkgerrors.Wrap(err, "problem with address")
		}
		r.Host = sh
		r.Port = sp
		r.srvListener = tlsListener
	}

	r.hasListeners = true
	return nil
}

func (r *RestAPI) Shutdown() {
	log.Printf("Stopping ReST API Service")
	if r.HTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		r.HTTPServer.SetKeepAlivesEnabled(false)
		if err := r.HTTPServer.Shutdown(ctx); err != nil {
			log.Warnf("Could not gracefully shutdown the server: %v\n", err)
		}
	}
	log.Printf("Stopped ReST API Service")
}
