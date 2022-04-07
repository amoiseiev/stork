# Development
# This file defines development-stage tasks,
# e.g., unit testing, lining, or debugging.

###############
### Backend ###
###############

go_codebase = GO_SERVER_CODEBASE +
        GO_AGENT_CODEBASE +
        GO_TOOL_CODEBASE

go_dev_codebase = go_codebase + [GO_SERVER_API_MOCK]

desc 'Build Stork backend continuously whenever source files change'
task :build_backend_live => go_codebase do
    Open3.pipeline(
        ['printf', '%s\\n', *go_codebase],
        ['entr', '-d', 'rake', 'build_backend']
    )
end

desc 'Check backend source code
    FIX - fix linting issues - default: false
'
task :lint_go => [GOLANGCILINT] + go_dev_codebase do
    opts = []
    if ENV["FIX"] == "true"
        opts += ["--fix"]
    end

    Dir.chdir("backend") do
        sh GOLANGCILINT, "run", *opts
    end
end

desc 'Format backend source code'
task :fmt_go => [GO] + go_codebase do
    Dir.chdir('backend') do
        sh GO, "fmt", "./..."
    end
end

desc 'Run backend unit and coverage tests
    SCOPE - Scope of the tests - default: all files
    TEST - Test name pattern to run - default: empty
    BENCHMARK - Execute benchmarks - default: false
    SHORT - Run short test rountine - default: false
    HEADLESS - Run in headless mode - default: false
    See "db_migrate" task for the database-related parameters
'
task :unittest_backend => [RICHGO, :db_remove_remaining, :db_migrate] + go_dev_codebase do
    scope = ENV["SCOPE"] || "./..."
    benchmark = ENV["BENCHMARK"] || "false"
    short = ENV["SHORT"] || "false"
    test_pattern = ENV["TEST"] || "false"

    opts = []

    if !ENV["TEST"].nil?
        opts += ["-run", ENV["TEST"]]
    end

    if benchmark == "true"
        opts += ["-bench=."]
    end

    if short == "true"
        opts += ["-short"]
    end

    with_cov_tests = scope == "./..." && ENV["TEST"].nil?

    if with_cov_tests
        opts += ["-coverprofile=coverage.out"]

        at_exit {
            sh "rm -f backend/coverage.out"
        }
    end

    Dir.chdir('backend') do
        sh RICHGO, "test", *opts, "-race", "-v", scope

        if with_cov_tests
            out = `"#{GO}" tool cover -func=coverage.out`
        
            puts out, ''

            problem = false
            out.each_line do |line|
                if line.start_with? 'total:' or line.include? 'api_mock.go'
                    next
                end
                items = line.gsub(/\s+/m, ' ').strip.split(" ")
                file = items[0]
                func = items[1]
                cov = items[2].strip()[0..-2].to_f
                ignore_list = ['DetectServices', 'RestartKea', 'Serve', 'BeforeQuery', 'AfterQuery',
                            'Identity', 'LogoutHandler', 'NewDatabaseSettings', 'ConnectionParams',
                            'Password', 'loggingMiddleware', 'GlobalMiddleware', 'Authorizer',
                            'Listen', 'Shutdown', 'SetupLogging', 'UTCNow', 'detectApps',
                            'prepareTLS', 'handleRequest', 'pullerLoop', 'Output', 'Collect',
                            'collectTime', 'collectResolverStat', 'collectResolverLabelStat',

                            # Those two are tested in backend/server/server_test.go, in TestCommandLineSwitches*
                            # However, due to how it's executed (calling external binary), it's not detected
                            # by coverage.
                            'ParseArgs', 'Bootstrap',

                            # this function requires interaction with user so it is hard to test
                            'getAgentAddrAndPortFromUser',

                            # this requires interacting with terminal
                            'GetSecretInTerminal',
                            ]
                if short == 'true'
                    ignore_list.concat(['setupRootKeyAndCert', 'setupServerKeyAndCert', 'SetupServerCerts',
                                    'ExportSecret'])
                end

                if cov < 35 and not ignore_list.include? func
                    puts "FAIL: %-80s %5s%% < 35%%" % ["#{file} #{func}", "#{cov}"]
                    problem = true
                end
            end

            if problem
                fail("\nFAIL: Tests coverage is too low, add some tests\n\n")
            end
        end
    end
end

desc 'Show backend coverage of unit tests in web browser
    See "db_migrate" task for the database-related parameters'
task :show_backend_cov => [GO, :unittest_backend] do
    if !ENV["SCOPE"].nil?
        fail "Environment variable SCOPE cannot be specified"
    end

    if !ENV["TEST"].nil?
        fail "Environment variable TEST cannot be specified"
    end

    puts "Warning: Coverage may not work under Chrome-like browsers; use Firefox if any problems occur."
    Dir.chdir('backend') do
        sh GO, "tool", "cover", "-html=coverage.out"
    end
end

##########################
### Backend- Debugging ###
##########################

desc 'Connect gdlv GUI Go debugger to waiting dlv debugger'
task :connect_dbg => GDLV do
  sh GDLV, "connect", "127.0.0.1:45678"
end

desc 'Run backend unit tests (debug mode)
    SCOPE - Scope of the tests - required
    HEADLESS - Run in headless mode - default: false
    See "db_migrate" task for the database-related parameters
'
task :unittest_backend_debug => [DLV, :db_remove_remaining, :db_migrate] + go_dev_codebase do
    if ENV["SCOPE"].nil?
        fail "Scope argument is required"
    end

    opts = []

    if ENV["HEADLESS"] == "true"
        opts = ["--headless", "-l", "0.0.0.0:45678"]
    end

    Dir.chdir('backend') do
        sh DLV, *opts, "test", ENV["SCOPE"]
    end
end

desc 'Run Stork Agent (debug mode)
    HEADLESS - run debugger in headless mode - default: false'
task :run_agent_debug => [DLV] + GO_AGENT_CODEBASE do
    opts = []

    if ENV["HEADLESS"] == "true"
        opts = ["--headless", "-l", "0.0.0.0:45678"]
    end

    Dir.chdir("backend/cmd/stork-agent") do
        sh DLV, *opts, "debug"
    end
end

desc "Run Stork Server (debug mode, no doc and UI)
    HEADLESS - run debugger in headless mode - default: false
    UI_MODE - WebUI mode to use, must be build separately - choose: 'production', 'testing' or unspecify
    DB_TRACE - trace SQL queries - default: false"
task :run_server_debug => [DLV, :pre_run_server] + GO_SERVER_CODEBASE do
    opts = []
    if ENV["HEADLESS"] == "true"
        opts = ["--headless", "-l", "0.0.0.0:45678"]
    end

    Dir.chdir("backend/cmd/stork-server") do
        sh DLV, *opts, "debug"
    end
end

################
### Frontend ###
################

desc 'Check frontend source code'
task :lint_ui => [NPX] + WEBUI_CODEBASE do
  Dir.chdir('webui') do
    sh NPX, "ng", "lint"
    sh NPX, "prettier", "--config", ".prettierrc", "--check", "**/*"
  end
end

desc 'Make frontend source code prettier'
task :fmt_ui => [NPX] + WEBUI_CODEBASE do
  Dir.chdir('webui') do
    sh NPX, "prettier", "--config", ".prettierrc", "--write", "**/*"
  end
end

desc 'Run unit tests for UI.
    TEST - globs of test files to include, relative to project root - default: unspecified
        There are 2 special cases:
            when a path to directory is provided, all spec files ending ".spec.@(ts|tsx)" will be included
            when a path to a file is provided, and a matching spec file exists it will be included instead
    DEBUG - run the tests in debug mode (no headless) - default: false
'
task :unittest_ui => [NPX] + WEBUI_CODEBASE do
    debug = "false"
    if ENV["DEBUG"] == "true"
        debug = "true"
    end

    opts = []
    if !ENV["TEST"].nil?
        opts += ["--include", ENV["TEST"]]
    end

    opts += ["--progress", debug]
    opts += ["--watch", debug]

    opts += ["--browsers"]
    if debug == "true"
        opts += ["Chrome"]
    else
        opts += ["ChromeNoSandboxHeadless"]
    end

    Dir.chdir('webui') do
        sh NPX, "ng", "test", *opts
    end
end

############################
### Frontend - Debugging ###
############################

desc 'Build Stork UI (testing mode)'
task :build_ui_debug => [WEBUI_DEBUG_DIRECTORY]

desc 'Build Stork UI (testing mode) continuously whenever source files change'
task :build_ui_live => [NPX] + WEBUI_CODEBASE do
    Dir.chdir('webui') do
        sh NPX, "ng", "build", "--watch"
    end
end

#####################
### Documentation ###
#####################

desc 'Builds Stork documentation continuously whenever source files change'
task :build_doc_live => DOC_CODEBASE do
    Open3.pipeline(
        ['printf', '%s\\n', *DOC_CODEBASE],
        ['entr', '-d', 'rake', 'build_doc']
    )
end

#################
### Simulator ###
#################

flask_requirements_file = "tests/sim/requirements.txt"
flask = File.expand_path("tools/python/bin/flask")
file flask => [flask_requirements_file] do
    sh "pip3", "install",
            *ci_opts,
            "--force-reinstall",
            "--upgrade",
            "--no-input",
            "--no-deps",
            "--target", ENV["PYTHONPATH"],
            "-r", flask_requirements_file
end

task :run_sim => [flask] do
    ENV["STORK_SERVER_URL"] = "http://localhost:8080"
    ENV["FLASK_ENV"] = "development"
    ENV["FLASK_APP"] = "sim.py"
    ENV["LC_ALL"]  = "C.UTF-8"
    ENV["LANG"] = "C.UTF-8"

    Dir.chdir('tests/sim') do
        sh flask, "run", "--host", "0.0.0.0", "--port", "5005"
    end
end

#############
### Other ###
#############

task :lint_git => [DANGER] do
    if ENV["CI"] == nil
        puts "Warning! You cannot run this command locally."
    end
    sh DANGER, "--fail-on-errors=true", "--new-comment"
end

desc 'Migrate (and create) database to the newest version
    DB_NAME - database name - default: env:POSTGRES_DB or storktest
    DB_HOST - database host - default: env:POSTGRES_ADDR or localhost
    DB_PORT - database port - default: 5432
    DB_USER - database user - default: env:POSTGRES_USER or storktest
    DB_PASSWORD - database password - default: env: POSTGRES_PASSWORD or storktest
    DB_TRACE - trace SQL queries - default: false
    DB_MAINTENANCE_NAME - maintanance DB name used to create the provided DB - default: postgres
    SUPPRESS_DB_MAINTENANCE - dont run creation DB operation - default: false
'
task :db_migrate => [TOOL_BINARY_FILE] do
    dbname = ENV["DB_NAME"] || ENV["POSTGRES_DB"] || "storktest"
    dbhost = ENV["DB_HOST"] || ENV["POSTGRES_ADDR"] || "localhost"
    dbport = ENV["DB_PORT"] || "5432"
    dbuser = ENV["DB_USER"] || ENV["POSTGRES_USER"] || "storktest"
    dbpass = ENV["DB_PASSWORD"] || ENV["POSTGRES_PASSWORD"] || "storktest"
    dbtrace = ENV["DB_TRACE"] || "false"
    dbmaintenance = ENV["DB_MAINTENANCE_NAME"] || "postgres"

    if dbhost.include? ':'
        dbhost, dbport = dbhost.split(':')
    end
    
    ENV["STORK_DATABASE_HOST"] = dbhost
    ENV["STORK_DATABASE_PORT"] = dbport
    ENV["STORK_DATABASE_USER_NAME"] = dbuser
    ENV["STORK_DATABASE_PASSWORD"] = dbpass
    ENV["STORK_DATABASE_NAME"] = dbname

    if dbtrace == "true"
        ENV["STORK_DATABASE_TRACE"] = "run"
    end
    
    ENV['PGPASSWORD'] = dbpass
    
    if ENV["SUPPRESS_DB_MAINTENANCE"] != "true"
        stdout, stderr, status = Open3.capture3 "psql",
            "-h", dbhost, "-p", dbport, "-U", dbuser, dbmaintenance, "-XtAc",
            "SELECT 1 FROM pg_database WHERE datname='#{dbname}'"

        if status != 0
            fail stderr
        end

        has_db = stdout.rstrip == "1"

        if !has_db
            sh "createdb",
                "-h", dbhost,
                "-p", dbport,
                "-U", dbuser,
                "-O", dbuser,
                dbname
        end
    else
        puts "DB maintenance suppressed (creating)"
    end

    sh TOOL_BINARY_FILE, "db-up",
        "-d", dbname,
        "-u", dbuser,
        "--db-host", dbhost,
        "-p", dbport
end

desc "Remove remaing test databases and users
    DB_NAME - database name - default: env:POSTGRES_DB or storktest
    DB_HOST - database host - default: env:POSTGRES_ADDR or localhost
    DB_PORT - database port - default: 5432
    DB_USER - database user - default: env:POSTGRES_USER or storktest
    DB_PASSWORD - database password - default: env: POSTGRES_PASSWORD or storktest
    SUPPRESS_DB_MAINTENANCE - dont run this stage (no removing) - default: false"
task :db_remove_remaining do
    if ENV["SUPPRESS_DB_MAINTENANCE"] == "true"
        puts "DB maintenance suppressed (removing)"
        next
    end

    dbname = ENV["DB_NAME"] || ENV["POSTGRES_DB"] || "storktest"
    dbhost = ENV["DB_HOST"] || ENV["POSTGRES_ADDR"] || "localhost"
    dbport = ENV["DB_PORT"] || "5432"
    dbuser = ENV["DB_USER"] || ENV["POSTGRES_USER"] || "storktest"
    dbpass = ENV["DB_PASSWORD"] || ENV["POSTGRES_PASSWORD"] || "storktest"
    dbmaintenance = ENV["DB_MAINTENANCE"] || "postgres"

    if dbhost.include? ':'
        dbhost, dbport = dbhost.split(':')
    end

    ENV['PGPASSWORD'] = dbpass

    psql_access_opts = [
        "-h", dbhost,
        "-p", dbport,
        "-U", dbuser
    ]

    psql_select_opts = [
        "-t",
        "-q",
        "-X",
    ]

    # Don't destroy the maintanance database
    dbname_pattern = "#{dbname}.*"
    if dbname == dbmaintenance
        dbname_pattern = "#{dbname}.+"
    end

    Open3.pipeline([
        "psql", *psql_select_opts, *psql_access_opts, dbmaintenance,
        "-c", "SELECT datname FROM pg_database WHERE datname ~ '#{dbname_pattern}'"
    ], [
        "xargs", "-P", "16", "-n", "1", "-r", "dropdb", *psql_access_opts 
    ])

    Open3.pipeline([
        "psql", *psql_select_opts, *psql_access_opts, dbmaintenance,
        "-c", "SELECT usename FROM pg_user WHERE usename ~ '#{dbuser}.+'"
    ], [
        "xargs", "-P", "16", "-n", "1", "-r", "dropuser", *psql_access_opts 
    ])
end