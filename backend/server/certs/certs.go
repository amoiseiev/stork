package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"isc.org/stork/pki"
	dbmodel "isc.org/stork/server/database/model"
)

const (
	SecretTypeCACert   = "CA cert"
	SecretTypeCAKey    = "CA key"
	SecretTypeSrvKey   = "server key"
	SecretTypeSrvCert  = "server cert"
	SecretTypeSrvToken = "server token"
)

// Generate server token and store it in database.  It is used during
// manual agent registration. This function uses crypto random numbers
// generator.
func GenerateServerToken(db *pg.DB) ([]byte, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	serverToken := make([]byte, 32)
	for i := range serverToken {
		maxValue := big.NewInt(int64(len(chars)))
		r, err := rand.Int(rand.Reader, maxValue)
		if err != nil {
			return nil, err
		}
		serverToken[i] = chars[r.Uint64()]
	}
	err := dbmodel.SetSecret(db, dbmodel.SecretServerToken, serverToken)
	if err != nil {
		return nil, err
	}
	return serverToken, nil
}

// Check if a root CA key and a root CA certs are present in db. If not generate them
// and store in database.
func setupRootKeyAndCert(db *pg.DB) (*ecdsa.PrivateKey, *x509.Certificate, []byte, error) {
	// check root key and root cert, if missing then generate them
	rootKeyPEM, err := dbmodel.GetSecret(db, dbmodel.SecretCAKey)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "problem getting CA key from database")
	}
	rootCertPEM, err := dbmodel.GetSecret(db, dbmodel.SecretCACert)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "problem getting CA cert from database")
	}

	var rootKey *ecdsa.PrivateKey
	var rootCert *x509.Certificate

	// no root key or no root cert so generate them
	if rootKeyPEM == nil || rootCertPEM == nil {
		certSerialNumber, err := dbmodel.GetNewCertSerialNumber(db)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot get new cert S/N")
		}
		rootKey, rootKeyPEM, rootCert, rootCertPEM, err = pki.GenCAKeyCert(certSerialNumber)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot generate root CA cert")
		}
		err = dbmodel.SetSecret(db, dbmodel.SecretCAKey, rootKeyPEM)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot store root CA key in database")
		}
		err = dbmodel.SetSecret(db, dbmodel.SecretCACert, rootCertPEM)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot store root CA key in database")
		}
		log.Printf("Generated root CA key and cert")
	} else {
		// key and cert present in db so just check them
		rootKeyPEMBlock, _ := pem.Decode(rootKeyPEM)
		rootKeyIf, err := x509.ParsePKCS8PrivateKey(rootKeyPEMBlock.Bytes)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot parse root CA key")
		}
		rootKey = rootKeyIf.(*ecdsa.PrivateKey)

		rootCertPEMBlock, _ := pem.Decode(rootCertPEM)
		rootCert, err = x509.ParseCertificate(rootCertPEMBlock.Bytes)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot parse root CA cert")
		}
		log.Printf("Root CA key and cert OK")
	}

	return rootKey, rootCert, rootCertPEM, nil
}

// Check if a server key and a server cert are present in db. If not generate them
// and store in database.
func setupServerKeyAndCert(db *pg.DB, rootKey *ecdsa.PrivateKey, rootCert *x509.Certificate) ([]byte, []byte, error) {
	// check server cert, if missing then generate it
	serverKeyPEM, err := dbmodel.GetSecret(db, dbmodel.SecretServerKey)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot get server key from database")
	}
	serverCertPEM, err := dbmodel.GetSecret(db, dbmodel.SecretServerCert)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot get server cert from database")
	}

	// no server key or no server cert so generate
	if serverKeyPEM == nil || serverCertPEM == nil {
		// get list of all host IP addresses that will be put to server cert
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot get interface addresses")
		}
		var srvIPs []net.IP
		var srvNames []string
		var resolver net.Resolver
		for _, addr := range addrs {
			ipAddr, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			srvIPs = append(srvIPs, ipAddr)

			// Lookup sometimes blocks on IPv6 loopback address on Debian 10.
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			names, err := resolver.LookupAddr(ctx, ipAddr.String())

			if err == nil {
				srvNames = append(srvNames, names...)
			}
		}
		if len(srvIPs) == 0 || len(srvNames) == 0 {
			return nil, nil, errors.Errorf("cannot find IP addresses on this host")
		}

		certSerialNumber, err := dbmodel.GetNewCertSerialNumber(db)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot get new cert S/N")
		}
		serverCertPEM, serverKeyPEM, err = pki.GenKeyCert("server", srvNames, srvIPs, certSerialNumber, rootCert, rootKey)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot generate key and cert for server")
		}
		err = dbmodel.SetSecret(db, dbmodel.SecretServerKey, serverKeyPEM)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot store server key in database")
		}
		err = dbmodel.SetSecret(db, dbmodel.SecretServerCert, serverCertPEM)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot store server cert in database")
		}
		log.Printf("Generated server key and cert")
	} else {
		// key and cert present in db so just check them
		serverKeyPEMBlock, _ := pem.Decode(serverKeyPEM)
		_, err := x509.ParsePKCS8PrivateKey(serverKeyPEMBlock.Bytes)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot parse server key")
		}

		serverCertPEMBlock, _ := pem.Decode(serverCertPEM)
		_, err = x509.ParseCertificate(serverCertPEMBlock.Bytes)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot parse server cert")
		}
		log.Printf("Server key and cert OK")
	}

	return serverKeyPEM, serverCertPEM, nil
}

// Check if there are root CA and server keys and certs, and server
// token in the database.  If they are missing then create them and
// store in the database. In the end return root CA cert, server key
// and cert, all in PEM format.
func SetupServerCerts(db *pg.DB) ([]byte, []byte, []byte, error) {
	log.Printf("Preparing certs, it may take several minutes")

	// setup root CA key and cert
	rootKey, rootCert, rootCertPEM, err := setupRootKeyAndCert(db)
	if err != nil {
		return nil, nil, nil, err
	}

	// setup server key and cert using root CA key and cert
	serverKeyPEM, serverCertPEM, err := setupServerKeyAndCert(db, rootKey, rootCert)
	if err != nil {
		return nil, nil, nil, err
	}

	// check server access token; if missing generate it
	serverToken, err := dbmodel.GetSecret(db, dbmodel.SecretServerToken)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "cannot get server token from database")
	}
	if serverToken == nil {
		_, err = GenerateServerToken(db)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot generate server token")
		}
	}

	return rootCertPEM, serverCertPEM, serverKeyPEM, nil
}

// Export a secret e.g. certificate or server token to stdout or to indicated file.
func ExportSecret(db *pg.DB, object string, filename string) error {
	var objDisplayName string
	switch object {
	case dbmodel.SecretCAKey:
		objDisplayName = SecretTypeCAKey
	case dbmodel.SecretCACert:
		objDisplayName = SecretTypeCACert
	case dbmodel.SecretServerKey:
		objDisplayName = SecretTypeSrvKey
	case dbmodel.SecretServerCert:
		objDisplayName = SecretTypeSrvCert
	case dbmodel.SecretServerToken:
		objDisplayName = SecretTypeSrvToken
	default:
		return errors.Errorf("requested unknown object '%s'", object)
	}

	content, err := dbmodel.GetSecret(db, object)
	if err != nil {
		return errors.Wrapf(err, "problem getting '%s' from database", objDisplayName)
	}
	if filename != "" {
		err := os.WriteFile(filename, content, 0o600)
		if err != nil {
			return err
		}
		log.Printf("%s saved to file: %s", objDisplayName, filename)
	} else {
		log.Printf("%s:\n%s", objDisplayName, string(content))
	}

	return nil
}

// Import a secret e.g. certificate or server token from stdin or from indicated file.
func ImportSecret(db *pg.DB, object string, filename string) error {
	var objDisplayName string
	switch object {
	case dbmodel.SecretCAKey:
		objDisplayName = SecretTypeCAKey
	case dbmodel.SecretCACert:
		objDisplayName = SecretTypeCACert
	case dbmodel.SecretServerKey:
		objDisplayName = SecretTypeSrvKey
	case dbmodel.SecretServerCert:
		objDisplayName = SecretTypeSrvCert
	case dbmodel.SecretServerToken:
		objDisplayName = SecretTypeSrvToken
	default:
		return errors.Errorf("indicated unknown object '%s'", object)
	}

	var content []byte
	var err error
	if filename != "" {
		content, err = os.ReadFile(filename)
		if err != nil {
			return err
		}
		log.Printf("%s loaded from %s file, length %d", objDisplayName, filename, len(content))
	} else {
		log.Printf("Reading %s from stdin", objDisplayName)
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		log.Printf("%s read from stdin, length %d", objDisplayName, len(content))
	}

	// Now we need to conduct verification if the content is sane.
	switch object {
	case dbmodel.SecretCAKey:
		objDisplayName = SecretTypeCAKey
		_, err = pki.ParsePrivateKey(content)
	case dbmodel.SecretCACert:
		objDisplayName = SecretTypeCACert
		_, err = pki.ParseCert(content)
	case dbmodel.SecretServerKey:
		objDisplayName = SecretTypeSrvKey
		_, err = pki.ParsePrivateKey(content)
	case dbmodel.SecretServerCert:
		objDisplayName = SecretTypeSrvCert
		_, err = pki.ParseCert(content)
	case dbmodel.SecretServerToken:
		objDisplayName = SecretTypeSrvToken
		if len(content) != 32 {
			return errors.Errorf("server token must be exactly 32 bytes long, token provided is %d bytes", len(content))
		}
	default:
		return errors.Errorf("unsupported object: %s", object)
	}

	if err != nil {
		return errors.Wrapf(err, "problem parsing the %s", objDisplayName)
	}

	// The value looks reasonable. Let's set it in the DB
	err = dbmodel.SetSecret(db, object, content)
	if err != nil {
		return errors.Wrapf(err, "problem setting '%s' in database", objDisplayName)
	}

	return nil
}
