# coding: utf-8

# Initialization
# This file contains the toolkits that
# aren't related to the source code.
# It means that they don't change very often
# and can be cached for later use.

require 'open3'

# Cross-platform way of finding an executable in the $PATH.
# Source: https://stackoverflow.com/a/5471032
#
#   which('ruby') #=> /usr/bin/ruby
def which(cmd)
    exts = ENV['PATHEXT'] ? ENV['PATHEXT'].split(';') : ['']
    ENV['PATH'].split(File::PATH_SEPARATOR).each do |path|
      exts.each do |ext|
        exe = File.join(path, "#{cmd}#{ext}")
        return exe if File.executable?(exe) && !File.directory?(exe)
      end
    end
    nil
end

### Recognize the operating system
uname=`uname -s`

case uname.rstrip
  when "Darwin"
    OS="macos"
  when "Linux"
    OS="linux"
  when "FreeBSD"
    OS="FreeBSD"
  else
    puts "ERROR: Unknown/unsupported OS: %s" % UNAME
    fail
  end

### Detect wget
if system("wget --version > /dev/null").nil?
    abort("wget is not installed on this system")
end
# extract wget version
wget_version = `wget --version | head -n 1 | sed -E 's/[^0-9]*([0-9]*\.[0-9]*)[^0-9]+.*/\1/g'`
# versions prior to 1.19 lack support for --retry-on-http-error
wget = ["wget", "--tries=inf", "--waitretry=3"]
if wget_version.empty? or wget_version >= "1.19"
    wget = wget + ["--retry-on-http-error=429,500,503,504"]
end

if ENV["CI"] == "true"
    wget = wget + ["-q"]
end
WGET = wget

### Define package versions
go_ver='1.17.5'
openapi_generator_ver='5.2.0'
goswagger_ver='v0.23.0'
protoc_ver='3.18.1'
protoc_gen_go_ver='v1.26.0'
protoc_gen_go_grpc='v1.1.0'
richgo_ver='v0.3.10'
mockery_ver='v1.0.0'
mockgen_ver='v1.6.0'
golangcilint_ver='1.33.0'
fpm_ver='1.14.1'
danger_ver='8.4.5'
danger_linter_ver='0.0.7'
danger_gitlab_ver='8.0.0'
yamlinc_ver='0.1.10'
node_ver='14.18.2'
dlv_ver='v1.8.1'
gdlv_ver='v1.7.0'
sphinx_ver='4.4.0'
bundler_ver='2.3.8'

# System-depends variables
case OS
when "macos"
  go_suffix="darwin-amd64"
  goswagger_suffix="darwin_amd64"
  protoc_suffix="osx-x86_64"
  node_suffix="darwin-x64"
  golangcilint_suffix="darwin-amd64"
  chrome_drv_suffix="mac64"
  puts "WARNING: MacOS is not officially supported, the provisions for building on MacOS are made"
  puts "WARNING: for the developers' convenience only."
when "linux"
  go_suffix="linux-amd64"
  goswagger_suffix="linux_amd64"
  protoc_suffix="linux-x86_64"
  node_suffix="linux-x64"
  golangcilint_suffix="linux-amd64"
  chrome_drv_suffix="linux64"
when "FreeBSD"
  goswagger_suffix=""
  puts "WARNING: There are no FreeBSD packages for GOSWAGGER"
  go_suffix="freebsd-amd64"
  # TODO: there are no protoc built packages for FreeBSD (at least as of 3.10.0)
  protoc_suffix=""
  puts "WARNING: There are no protoc packages built for FreeBSD"
  node_suffix="node-v14.18.2.tar.xz"
  golangcilint_suffix="freebsd-amd64"
  chrome_drv_suffix=""
  puts "WARNING: There are no chrome drv packages built for FreeBSD"
else
  puts "ERROR: Unknown/unsupported OS: %s" % UNAME
  fail
end

### Detect Chrome
# CHROME_BIN is required for UI unit tests and system tests. If it is
# not provided by a user, try to locate Chrome binary and set
# environment variable to its location.
if !ENV['CHROME_BIN']
    chrome_locations = []
    if OS == 'linux'
      chrome_locations = ['/usr/bin/chromium-browser', '/snap/bin/chromium', '/usr/bin/chromium']
    elsif OS == 'macos'
      chrome_locations = ["/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome"]
    end
    # For each possible location check if the binary exists.
    chrome_locations.each do |loc|
      if File.exist?(loc)
        # Found Chrome binary.
        ENV['CHROME_BIN'] = loc
        break
      end
    end
end

### Define dependencies

# Directories
tools_dir = File.expand_path('tools')
directory tools_dir

node_dir = File.join(tools_dir, "nodejs")
directory node_dir

go_tools_dir = File.join(tools_dir, "golang")
gopath = File.join(go_tools_dir, "gopath")
directory go_tools_dir
directory gopath
file go_tools_dir => [gopath]

ruby_tools_dir = File.join(tools_dir, "ruby")
directory ruby_tools_dir

ruby_tools_bin_dir = File.join(ruby_tools_dir, "bin")
directory ruby_tools_bin_dir

python_tools_dir = File.join(tools_dir, "python")
directory python_tools_dir

pythonpath = File.join(python_tools_dir, "lib")
directory pythonpath

# Automatically created directories by tools
ruby_tools_gems_dir = File.join(ruby_tools_dir, "gems")
node_bin_dir = File.join(node_dir, "bin")
goroot = File.join(go_tools_dir, "go")
gobin = File.join(goroot, "bin")

# Environment variables
ENV["GEM_HOME"] = ruby_tools_dir
ENV["GOROOT"] = goroot
ENV["GOPATH"] = gopath
ENV["GOBIN"] = gobin
ENV["PATH"] = "#{node_bin_dir}:#{tools_dir}:#{gobin}:#{ENV["PATH"]}"
ENV["PYTHONPATH"] = pythonpath

# Toolkits
FPM = File.join(ruby_tools_bin_dir, "fpm")
file FPM => [ruby_tools_dir, ruby_tools_bin_dir] do
    sh "gem", "install",
            "--minimal-deps",
            "--no-document",
            "--install-dir", ruby_tools_dir,
            "fpm:#{fpm_ver}"

    if !File.exists? FPM
        # Workaround for old Ruby versions
        sh "ln", "-s", File.join(ruby_tools_gems_dir, "fpm-#{fpm_ver}", "bin", "fpm"), FPM
    end

    sh FPM, "--version"
end

BUNDLER = File.join(ruby_tools_bin_dir, "bundler")
file BUNDLER => [ruby_tools_dir, ruby_tools_bin_dir] do
    sh "gem", "install",
            "--minimal-deps",
            "--no-document",
            "--install-dir", ruby_tools_dir,
            "bundler:#{bundler_ver}"

    if !File.exists? BUNDLER
        # Workaround for old Ruby versions
        sh "ln", "-s", File.join(ruby_tools_gems_dir, "bundler-#{bundler_ver}", "exe", "bundler"), BUNDLER
        sh "ln", "-s", File.join(ruby_tools_gems_dir, "bundler-#{bundler_ver}", "exe", "bundle"), File.join(ruby_tools_bin_dir, "bundle")
    end

    sh BUNDLER, "--version"
end

danger_gitlab_dir = File.join(ruby_tools_gems_dir, "danger-gitlab-#{danger_gitlab_ver}")
file danger_gitlab_dir => [ruby_tools_dir, ruby_tools_bin_dir] do
    sh "gem", "install",
        "--minimal-deps",
        "--no-document",
        "--install-dir", ruby_tools_dir,
        "danger:#{danger_ver}",
        "danger-gitlab:#{danger_gitlab_ver}"
    sh "touch", danger_gitlab_dir
end

danger_commit_linter_dir = File.join(ruby_tools_gems_dir, "danger-commit_lint-#{danger_linter_ver}")
file danger_commit_linter_dir => [ruby_tools_dir] do
    sh "gem", "install",
        "--minimal-deps",
        "--no-document",
        "--install-dir", ruby_tools_dir,
        "danger-commit_lint:#{danger_linter_ver}"
    sh "touch", danger_commit_linter_dir
end

DANGER = File.join(ruby_tools_bin_dir, "danger")
file DANGER => [ruby_tools_bin_dir, danger_gitlab_dir, danger_commit_linter_dir, BUNDLER] do
    if !File.exists? DANGER
        # Workaround for old Ruby versions
        sh "ln", "-s", File.join(ruby_tools_gems_dir, "danger-#{danger_ver}", "bin", "danger"), DANGER
    end

    sh "touch", DANGER
    sh DANGER, "--version"
end

NPM = File.join(node_bin_dir, "npm")
file NPM => [node_dir] do
    Dir.chdir(node_dir) do
        sh *WGET, "https://nodejs.org/dist/v#{node_ver}/node-v#{node_ver}-#{node_suffix}.tar.xz", "-O", "node.tar.xz"
        sh "tar", "-Jxf", "node.tar.xz", "--strip-components=1"
        sh "rm", "node.tar.xz"
    end
    sh NPM, "--version"
end

NPX = File.join(node_bin_dir, "npx")
file NPX => [NPM] do
    sh NPX, "--version"
end

YAMLINC = File.join(node_dir, "node_modules", "lib", "node_modules", "yamlinc", "bin", "yamlinc")
file YAMLINC => [NPM] do
    ci_opts = []
    if ENV["CI"] == "true"
        ci_opts += ["--no-audit", "--no-progress"]
    end

    sh NPM, "install",
            "-g",
            *ci_opts,
            "--prefix", "#{node_dir}/node_modules",
            "yamlinc@#{yamlinc_ver}"
    sh "touch", YAMLINC
    sh YAMLINC, "--version"
end

CHROME_DRV = File.join(tools_dir, "chromedriver")
file CHROME_DRV => [tools_dir] do
    if !ENV['CHROME_BIN']
        puts "Missing Chrome/Chromium binary. It is required for UI unit tests and system tests."
        next
    end

    chrome_version = `"#{ENV['CHROME_BIN']}" --version | cut -d" " -f2 | tr -d -c 0-9.`
    chrome_drv_version = chrome_version

    if chrome_version.include? '85.'
        chrome_drv_version = '85.0.4183.87'
    elsif chrome_version.include? '86.'
        chrome_drv_version = '86.0.4240.22'
    elsif chrome_version.include? '87.'
        chrome_drv_version = '87.0.4280.20'
    elsif chrome_version.include? '90.'
        chrome_drv_version = '90.0.4430.72'
    elsif chrome_version.include? '92.'
        chrome_drv_version = '92.0.4515.159'
    elsif chrome_version.include? '93.'
        chrome_drv_version = '93.0.4577.63'
    elsif chrome_version.include? '94.'
        chrome_drv_version = '94.0.4606.61' 
    end

    Dir.chdir(tools_dir) do
        sh *WGET, "https://chromedriver.storage.googleapis.com/#{chrome_drv_version}/chromedriver_#{chrome_drv_suffix}.zip", "-O", "chromedriver.zip"
        sh "unzip", "-o", "chromedriver.zip"
        sh "rm", "chromedriver.zip"
    end

    sh CHROME_DRV, "--version"
    sh "chromedriver", "--version"  # From PATH
end

OPENAPI_GENERATOR = File.join(tools_dir, "openapi-generator-cli.jar")
file OPENAPI_GENERATOR => tools_dir do
    sh *WGET, "https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/#{openapi_generator_ver}/openapi-generator-cli-#{openapi_generator_ver}.jar", "-O", OPENAPI_GENERATOR
end

GO = File.join(gobin, "go")
file GO => [go_tools_dir] do
    Dir.chdir(go_tools_dir) do
        sh *WGET, "https://dl.google.com/go/go#{go_ver}.#{go_suffix}.tar.gz", "-O", "go.tar.gz"
        sh "tar", "-zxf", "go.tar.gz" 
        sh "rm", "go.tar.gz"
    end
    sh GO, "version"
end

GOSWAGGER = File.join(go_tools_dir, "goswagger")
file GOSWAGGER => [go_tools_dir] do
    sh *WGET, "https://github.com/go-swagger/go-swagger/releases/download/#{goswagger_ver}/swagger_#{goswagger_suffix}", "-O", GOSWAGGER
    sh "chmod", "u+x", GOSWAGGER
    sh GOSWAGGER, "version"
end

PROTOC = File.join(go_tools_dir, "protoc")
file PROTOC => [go_tools_dir] do
    Dir.chdir(go_tools_dir) do
        sh *WGET, "https://github.com/protocolbuffers/protobuf/releases/download/v#{protoc_ver}/protoc-#{protoc_ver}-#{protoc_suffix}.zip", "-O", "protoc.zip"
        sh "unzip", "-j", "protoc.zip", "bin/protoc"
        sh "rm", "protoc.zip"
    end
    sh PROTOC, "--version"
end

PROTOC_GEN_GO = File.join(gobin, "protoc-gen-go")
file PROTOC_GEN_GO => [GO] do
    sh GO, "install", "google.golang.org/protobuf/cmd/protoc-gen-go@#{protoc_gen_go_ver}"
    sh PROTOC_GEN_GO, "--version"
end

PROTOC_GEN_GO_GRPC = File.join(gobin, "protoc-gen-go-grpc")
file PROTOC_GEN_GO_GRPC => [GO] do
    sh GO, "install", "google.golang.org/grpc/cmd/protoc-gen-go-grpc@#{protoc_gen_go_grpc}"
    sh PROTOC_GEN_GO_GRPC, "--version"
end

GOLANGCILINT = File.join(go_tools_dir, "golangci-lint")
file GOLANGCILINT => [go_tools_dir] do
    Dir.chdir(go_tools_dir) do
        sh *WGET, "https://github.com/golangci/golangci-lint/releases/download/v#{golangcilint_ver}/golangci-lint-#{golangcilint_ver}-#{golangcilint_suffix}.tar.gz", "-O", "golangci-lint.tar.gz"
        sh "mkdir", "tmp"
        sh "tar", "-zxf", "golangci-lint.tar.gz", "-C", "tmp", "--strip-components=1"
        sh "mv", "tmp/golangci-lint", "."
        sh "rm", "-rf", "tmp"
        sh "rm", "-f", "golangci-lint.tar.gz"
    end
    sh GOLANGCILINT, "--version"
end

RICHGO = "#{gobin}/richgo"
file RICHGO => [GO] do
    sh GO, "install", "github.com/kyoh86/richgo@#{richgo_ver}"
    sh RICHGO, "version"
end

MOCKERY = "#{gobin}/mockery"
file MOCKERY => [GO] do
    sh GO, "get", "github.com/vektra/mockery/.../@#{mockery_ver}"
    sh MOCKERY, "--version"
end

MOCKGEN = "#{gobin}/mockgen"
file MOCKGEN => [GO] do
    sh GO, "install", "github.com/golang/mock/mockgen@#{mockgen_ver}"
    sh MOCKGEN, "--version"
end

DLV = "#{gobin}/dlv"
file DLV => [GO] do
    sh GO, "install", "github.com/go-delve/delve/cmd/dlv@#{dlv_ver}"
    sh DLV, "version"
end

GDLV = "#{gobin}/gdlv"
file GDLV => [GO] do
    sh GO, "install", "github.com/aarzilli/gdlv@#{gdlv_ver}"
    if !File.file?(GDLV)
        fail
    end
end

sphinx_path = File.expand_path("tools/python/bin/sphinx-build")
if ENV["OLD_CI"] == "yes"
    python_location = which("python3")
    python_bin_dir = File.dirname(python_location)
    sphinx_path = File.join(python_bin_dir, "sphinx-build")
end
sphinx_requirements_file = File.expand_path("init_debs/sphinx.txt", __dir__)
SPHINX_BUILD = sphinx_path
file SPHINX_BUILD => [python_tools_dir, sphinx_requirements_file] do
    if ENV["OLD_CI"] == "yes"
        sh "touch", "-c", SPHINX_BUILD
        next
    end
    Rake::Task["pip_install"].invoke(sphinx_requirements_file)
    sh SPHINX_BUILD, "--version"
end

######################
### Internal tasks ###
######################

task :pip_install, [:requirements_file] => [python_tools_dir, pythonpath] do |t, args|
    ci_opts = []
    if ENV["CI"] == "true"
        ci_opts += ["--no-cache-dir"]
    end
    # Fix for Keyring error with pip. https://github.com/pypa/pip/issues/7883
    ENV["PYTHON_KEYRING_BACKEND"] = "keyring.backends.null.Keyring"
    sh "pip3", "install",
            *ci_opts,
            "--force-reinstall",
            "--upgrade",
            "--no-input",
            "--no-deps",
            # In Python 3.9 ang higher the target option can be used
            "--prefix", python_tools_dir,
            # "--target", python_tools_dir
            "-r", args.requirements_file

    # pip install "--target" option doesn't include bin
    # directory for Python < 3.9 version.
    # To workaround this problem, the "--prefix" option
    # is used, but it causes the library path to contain
    # the Python version.
    python_version_out = `python3 --version`
    python_version = (python_version_out.split)[1]
    python_major_minor = python_version.split(".")[0,2].join(".")
    site_packages_dir = File.join(python_tools_dir, "lib", "python" + python_major_minor, "site-packages")
    sh "cp", "-a", site_packages_dir + "/.", pythonpath
    sh "rm", "-rf", site_packages_dir
end

#############
### Tasks ###
#############

desc 'Check that the system-level dependencies are available (install nothing)'
task :check_env => [] do
    sh *WGET, "--version"
    if which("java") == nil
        puts "Missing java"
        fail
    else
        puts "java ready"
    end
    sh "rake", "--version"
    sh "python3", "--version"
    sh "pip3", "--version"
    if which("entr") == nil
        puts "Missing entr"
        fail
    else
        puts "entr ready"
    end
    sh "git", "--version"
    sh "createdb", "--version"
    sh "psql", "--version"
    sh "dropdb", "--version"

    sh ENV['CHROME_BIN'], "--version"
    sh "chromedriver", "--version"

    sh BUNDLER, "--version"
    sh YAMLINC, "--version"
    sh GOSWAGGER, "version"
    sh PROTOC, "--version"
    sh PROTOC_GEN_GO, "--version"
    sh PROTOC_GEN_GO_GRPC, "--version"
    sh RICHGO, "version"
    sh MOCKERY, "--version"
    sh MOCKGEN, "--version"
    sh DLV, "version"
    sh GDLV, "version"
    sh FPM, "--version"
    sh DANGER, "--version"
    sh GOLANGCILINT, "--version"
    sh SPHINX_BUILD, "--version"
    sh NPM, "--version"
    sh NPX, "--version"

    puts "All dependencies are OK!"
end

desc 'Install system-level dependencies'
task :prepare_env => [SPHINX_BUILD, FPM, DANGER, BUNDLER, NPM, NPX, YAMLINC, OPENAPI_GENERATOR, GO, GOSWAGGER, PROTOC, GOLANGCILINT, PROTOC_GEN_GO, PROTOC_GEN_GO_GRPC, RICHGO, MOCKERY, MOCKGEN, DLV, GDLV] do
    puts "Preparing complete!"
end