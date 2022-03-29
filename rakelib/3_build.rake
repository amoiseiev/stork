# coding: utf-8

# Build 
# This file is responsible for building (compiling)
# the binaries and other artifacts (docs, bundles). 

############
### Date ###
############

require 'date'

now = Time.now
build_date = now.strftime("%Y-%m-%d %H:%M")

if ENV['STORK_BUILD_TIMESTAMP']
    TIMESTAMP = ENV['STORK_BUILD_TIMESTAMP']
else
    TIMESTAMP = now.strftime("%y%m%d%H%M%S")
end

#####################
### Documentation ###
#####################

ARM_DIRECTORY = "doc/_build/html"
file ARM_DIRECTORY => DOC_CODEBASE + [SPHINX_BUILD] do
    sh SPHINX_BUILD, "-M", "html", "doc/", "doc/_build", "-v", "-E", "-a", "-W", "-j", "2"
    sh "touch", ARM_DIRECTORY
end
CLEAN.append *FileList[ARM_DIRECTORY + "/**/*"], ARM_DIRECTORY

TOOL_MAN_FILE = "doc/man/stork-tool.8"
file TOOL_MAN_FILE => DOC_CODEBASE + [SPHINX_BUILD] do
    sh SPHINX_BUILD, "-M", "man", "doc/", "doc/", "-v", "-E", "-a", "-W", "-j", "2"
    sh "touch", TOOL_MAN_FILE, AGENT_MAN_FILE, SERVER_MAN_FILE
end

AGENT_MAN_FILE = "doc/man/stork-agent.8"
file AGENT_MAN_FILE => [TOOL_MAN_FILE]

SERVER_MAN_FILE = "doc/man/stork-server.8"
file SERVER_MAN_FILE => [TOOL_MAN_FILE]

man_files = FileList[SERVER_MAN_FILE, AGENT_MAN_FILE, TOOL_MAN_FILE]
CLEAN.append *man_files

################
### Frontend ###
################

file WEBUI_DIST_DIRECTORY = "webui/dist/stork"
file WEBUI_DIST_DIRECTORY => WEBUI_CODEBASE + [NPX] do
    Dir.chdir("webui") do
        sh NPX, "ng", "build", "--configuration", "production"
    end
end

file WEBUI_DIST_ARM_DIRECTORY = "webui/dist/stork/assets/arm"
file WEBUI_DIST_ARM_DIRECTORY => [ARM_DIRECTORY] do
    sh "cp", "-a", ARM_DIRECTORY, WEBUI_DIST_ARM_DIRECTORY
    sh "touch", WEBUI_DIST_ARM_DIRECTORY
end

file WEBUI_DEBUG_DIRECTORY = "webui/dist/stork-debug"
file WEBUI_DEBUG_DIRECTORY => WEBUI_CODEBASE + [NPX] do
    Dir.chdir("webui") do
        sh NPX, "ng", "build"
    end
end

CLEAN.append *FileList["webui/dist/**/*"], "webui/dist"
CLOBBER.append "webui/.angular"

###############
### Backend ###
###############

AGENT_BINARY_FILE = "backend/cmd/stork-agent/stork-agent"
file AGENT_BINARY_FILE => GO_AGENT_CODEBASE + [GO] do
    Dir.chdir("backend/cmd/stork-agent") do
        sh GO, "build", "-ldflags=\"-X 'isc.org/stork.BuildDate=#{build_date}'\""
    end
    puts "Stork Agent build date: #{build_date} (timestamp: #{TIMESTAMP})"
end
CLEAN.append AGENT_BINARY_FILE

SERVER_BINARY_FILE = "backend/cmd/stork-server/stork-server"
file SERVER_BINARY_FILE => GO_SERVER_CODEBASE + [GO] do
    sh "rm", "-f", GO_SERVER_API_MOCK
    Dir.chdir("backend/cmd/stork-server") do
        sh GO, "build", "-ldflags=\"-X 'isc.org/stork.BuildDate=#{build_date}'\""
    end
    puts "Stork Server build date: #{build_date} (timestamp: #{TIMESTAMP})"
end
CLEAN.append SERVER_BINARY_FILE

TOOL_BINARY_FILE = "backend/cmd/stork-tool/stork-tool"
file TOOL_BINARY_FILE => GO_TOOL_CODEBASE + [GO] do
    Dir.chdir("backend/cmd/stork-tool") do
        sh GO, "build", "-ldflags=\"-X 'isc.org/stork.BuildDate=#{build_date}'\""
    end
    puts "Stork Tool build date: #{build_date} (timestamp: #{TIMESTAMP})"
end
CLEAN.append TOOL_BINARY_FILE

#############
### Tasks ###
#############

## Documentation

task :build_doc => man_files + [ARM_DIRECTORY]

task :rebuild_doc do
    sh "touch", "doc"
    Rake::Task["build_doc"].invoke()
end

## Stork Server

desc "Build Stork Server from sources"
task :build_server => [SERVER_BINARY_FILE]

desc "Rebuild Stork Server from sources"
task :rebuild_server do
    sh "touch", "backend/cmd/stork-server"
    Rake::Task["build_server"].invoke()
end

# Internal task that configure environment variables for server
task :pre_run_server, [:dbtrace, :ui_mode] do |t, args|
    if args.dbtrace == "true"
        ENV["STORK_DATABASE_TRACE"] = "run"
    end

    use_testing_ui = false
    # If the UI mode is not provided then detect it
    if args.ui_mode == nil
        # Enable testing mode if live build UI is active
        use_testing_ui = system "pgrep", "-f", "ng build --watch"
        # Enable testing mode if testing dir is newer then production dir
        if use_testing_ui == true
            puts "Using testing UI - live UI build is active"
        else
            production_time = File.mtime(WEBUI_DIST_DIRECTORY)
            testing_time = File.mtime(WEBUI_DEBUG_DIRECTORY)
            use_testing_ui = testing_time > production_time
            puts "Using testing UI - testing UI is newer than production"
        end
    elsif args.ui_mode == "testing"
        # Check if user manually forces the UI mode
        use_testing_ui = true
        puts "Using testing UI - user choice"
    elsif args.ui_mode != "production"
        puts "Invalid UI mode - choose 'production' or 'testing'"
        fail
    end
    if use_testing_ui
        ENV["STORK_REST_STATIC_FILES_DIR"] = "./webui/dist/stork-debug"
    end

    ENV["STORK_SERVER_ENABLE_METRICS"] = "true"
end

desc "Run Stork Server (release mode)"
task :run_server, [:dbtrace, :ui_mode] => [SERVER_BINARY_FILE, :pre_run_server] do
    sh SERVER_BINARY_FILE
end

## Stork Agent

desc "Build Stork Agent from sources"
task :build_agent => [AGENT_BINARY_FILE]

desc "Rebuild Stork Agent from sources"
task :rebuild_agent do
    sh "touch", "backend/cmd/stork-agent"
    Rake::Task["build_agent"].invoke()
end

desc "Run Stork Agent (release mode)"
task :run_agent, [:port] => [AGENT_BINARY_FILE] do |t, args|
    args.with_defaults(:port => "8888")
    sh AGENT_BINARY_FILE, "--port", args.port
end

## Stork Tool

desc "Build Stork Tool from sources"
task :build_tool => [TOOL_BINARY_FILE]

desc "Rebuild Stork Tool from sources"
task :rebuild_tool do
    sh "touch", "backend/cmd/stork-tool"
    Rake::Task["build_tool"].invoke()
end

## Stork UI

desc "Build Web UI (production mode)"
task :build_ui => [WEBUI_DIST_DIRECTORY, WEBUI_DIST_ARM_DIRECTORY]

desc "Rebuild Web UI (production mode)"
task :rebuild_ui do
    sh "touch", "webui"
    Rake::Task["build_ui"].invoke()
end

## Shorthands

desc "Build Stork Backend (Server, Agent, Tool)"
task :build_backend => [:build_server, :build_agent, :build_tool]

desc "Build all Stork components (Server, Agent, Tool, UI, doc)"
task :build_all => [:build_backend, :build_doc, :build_ui]