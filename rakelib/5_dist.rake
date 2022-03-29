# Distribution
# This file builds the distribution packages
# and install the application.

##############
### Common ###
##############

def get_pkg_type()
    begin
        _, _, status = Open3.capture3("rpm", "-q", "-a")
        if status.success?
            return "rpm"
        end
    rescue Exception
        # Command not exist
    end

    begin
        _, _, status = Open3.capture3("dpkg", "-l")
        if status.success?
            return "deb"
        end
    rescue Exception
        # Command not exist
    end

    puts "Unknown package type for current OS."
    fail
end

pkgs_dir = "dist/pkgs"
directory pkgs_dir

CLEAN.append *FileList["dist/**/*"], "dist"

##############
### Agent ###
##############

agent_dist_bin_dir = "dist/agent/usr/bin"
directory agent_dist_bin_dir
agent_dist_bin_file = File.join(agent_dist_bin_dir, "stork-agent")
file agent_dist_bin_file => [agent_dist_bin_dir, AGENT_BINARY_FILE] do
    sh "cp", "-a", AGENT_BINARY_FILE, agent_dist_bin_file
end

agent_dist_man_dir = "dist/agent/man/man8"
directory agent_dist_man_dir
agent_dist_man_file = File.join(agent_dist_man_dir, "stork-agent.8")
file agent_dist_man_file => [agent_dist_man_dir, AGENT_MAN_FILE] do
    sh "cp", "-a",  AGENT_MAN_FILE, agent_dist_man_file
end

agent_dist_system_dir = "dist/agent/lib/systemd/system/"
directory agent_dist_system_dir
agent_dist_system_service_file = File.join(agent_dist_system_dir, "isc-stork-agent.service")
file agent_dist_system_service_file => [agent_dist_system_dir, "etc/isc-stork-agent.service"] do
    sh "cp", "-a", "etc/isc-stork-agent.service", agent_dist_system_service_file
end

agent_etc_files = FileList["etc/agent.env", "etc/agent-credentials.json.template"]
agent_dist_etc_dir = "dist/agent/etc/stork"
file agent_dist_etc_dir => agent_etc_files do
    sh "mkdir", "-p", agent_dist_etc_dir
    agent_etc_files.each do |file|
        FileUtils.cp(file, agent_dist_etc_dir)
    end
    sh "touch", agent_dist_etc_dir
end

agent_dist_dir = "dist/agent"
directory agent_dist_dir
file agent_dist_dir => [agent_dist_bin_file, agent_dist_man_file, agent_dist_system_service_file, agent_dist_etc_dir]

agent_hooks = FileList["etc/isc-stork-agent.post*", "etc/isc-stork-agent.pre*"]

AGENT_PACKAGE_STUB_FILE = File.join(pkgs_dir, "agent-builded.pkg")
file AGENT_PACKAGE_STUB_FILE => [FPM, agent_dist_dir, pkgs_dir] + agent_hooks do
    ENV["PKG_NAME"] = "agent"
    Rake::Task["clean_pkgs"].invoke()

    version = `#{AGENT_BINARY_FILE} --version`.rstrip
    pkg_type = get_pkg_type()

    agent_dist_dir_abs = File.expand_path(agent_dist_dir)

    Dir.chdir(pkgs_dir) do
        sh FPM,
            "-C", agent_dist_dir_abs,
            "-n", "isc-stork-agent",
            "-s", "dir",
            "-t", pkg_type,
            "-v", "#{version}.#{TIMESTAMP}",
            "--after-install", "../../etc/isc-stork-agent.postinst",
            "--after-remove",  "../../etc/isc-stork-agent.postrm",
            "--before-remove", "../../etc/isc-stork-agent.prerm",
            "--config-files", "etc/stork/agent.env",
            "--config-files", "etc/stork/agent-credentials.json.template",
            "--description", "ISC Stork Agent",
            "--license", "MPL 2.0",
            "--url", "https://gitlab.isc.org/isc-projects/stork/",
            "--vendor", "Internet Systems Consortium, Inc."
    end
    sh "touch", AGENT_PACKAGE_STUB_FILE
end

##############
### Server ###
##############

server_dist_bin_dir = "dist/server/usr/bin"
directory server_dist_bin_dir
server_dist_bin_file = File.join(server_dist_bin_dir, "stork-server")
file server_dist_bin_file => [server_dist_bin_dir, SERVER_BINARY_FILE] do
    sh "cp", "-a", SERVER_BINARY_FILE, server_dist_bin_file
end
tool_dist_bin_file = File.join(server_dist_bin_dir, "stork-tool")
file tool_dist_bin_file => [server_dist_bin_dir, TOOL_BINARY_FILE] do
    sh "cp", "-a", TOOL_BINARY_FILE, tool_dist_bin_file
end

server_dist_man_dir = "dist/server/man/man8"
directory server_dist_man_dir
server_dist_man_file = File.join(server_dist_man_dir, "stork-server.8")
file server_dist_man_file => [server_dist_man_dir, SERVER_MAN_FILE] do
    sh "cp", "-a", SERVER_MAN_FILE, server_dist_man_file
end
tool_dist_man_file = File.join(server_dist_man_dir, "stork-tool.8")
file tool_dist_man_file => [server_dist_man_dir, TOOL_MAN_FILE] do
    sh "cp", "-a", TOOL_MAN_FILE, tool_dist_man_file
end

server_dist_system_dir = "dist/server/lib/systemd/system/"
directory server_dist_system_dir
server_dist_system_service_file = File.join(server_dist_system_dir, "isc-stork-server.service")
file server_dist_system_service_file => [server_dist_system_dir, "etc/isc-stork-server.service"] do
    sh "cp", "-a", "etc/isc-stork-server.service", server_dist_system_service_file
end

server_etc_files = FileList["etc/server.env"]
server_dist_etc_dir = "dist/server/etc/stork"
file server_dist_etc_dir => server_etc_files do
    sh "mkdir", "-p", server_dist_etc_dir
    server_etc_files.each do |file|
        FileUtils.cp(file, server_dist_etc_dir)
    end
    sh "touch", server_dist_etc_dir
end

server_examples_dir = "dist/server/usr/share/stork/examples"
directory server_examples_dir

server_grafana_examples_dir = File.join(server_examples_dir, "grafana")
file server_grafana_examples_dir => FileList["grafana/*.json"] do
    sh "mkdir", "-p", server_grafana_examples_dir
    sh "cp", "-a", *FileList["grafana/*.json"], server_grafana_examples_dir
    sh "touch", server_grafana_examples_dir
end

server_nginx_example_file = File.join(server_examples_dir, "nginx-stork.conf")
file server_nginx_example_file => ["etc/nginx-stork.conf", server_examples_dir] do
    sh "cp", "-a", "etc/nginx-stork.conf", server_examples_dir
end

server_www_dir = "dist/server/usr/share/stork/www"
file server_www_dir => [WEBUI_DIST_DIRECTORY, WEBUI_DIST_ARM_DIRECTORY] do
    sh "mkdir", "-p", server_www_dir
    sh "cp", "-a", *FileList[File.join(WEBUI_DIST_DIRECTORY, "*")], server_www_dir
    sh "touch", server_www_dir
end

server_dist_dir = "dist/server"
directory server_dist_dir
file server_dist_dir => [
    server_dist_bin_file,
    tool_dist_bin_file,
    tool_dist_man_file,
    server_dist_man_file,
    server_dist_system_service_file,
    server_dist_etc_dir,
    server_nginx_example_file, 
    server_grafana_examples_dir,
    server_www_dir
]

server_hooks = FileList["etc/isc-stork-server.post*", "etc/isc-stork-server.pre*"]

SERVER_PACKAGE_STUB_FILE = File.join(pkgs_dir, "server-builded.pkg")
file SERVER_PACKAGE_STUB_FILE => [FPM, server_dist_dir, pkgs_dir] + server_hooks do
    ENV["PKG_NAME"] = "server"
    Rake::Task["clean_pkgs"].invoke()

    version = `#{SERVER_BINARY_FILE} --version`.rstrip
    pkg_type = get_pkg_type()

    server_dist_dir_abs = File.expand_path(server_dist_dir)

    Dir.chdir(pkgs_dir) do
        sh FPM,
            "-C", server_dist_dir_abs,
            "-n", "isc-stork-server",
            "-s", "dir",
            "-t", pkg_type,
            "-v", "#{version}.#{TIMESTAMP}",
            "--after-install", "../../etc/isc-stork-server.postinst",
            "--after-remove",  "../../etc/isc-stork-server.postrm",
            "--before-remove", "../../etc/isc-stork-server.prerm",
            "--config-files", "etc/stork/server.env",
            "--description", "ISC Stork Server",
            "--license", "MPL 2.0",
            "--url", "https://gitlab.isc.org/isc-projects/stork/",
            "--vendor", "Internet Systems Consortium, Inc."
    end
    sh "touch", SERVER_PACKAGE_STUB_FILE
end

#############
### Tasks ###
#############

desc "Clean all packages of a given kind (agent or server)
    PKG_NAME - package name - choice: 'deb' or 'rpm', required
"
task :clean_pkgs do
    if ENV["PKG_NAME"].nil?
        fail "Environment variable PKG_NAME not specified"
    end
    pkgs = FileList[File.join(pkgs_dir, "isc-stork-#{ENV["PKG_NAME"]}*")]
    stub = File.join(pkgs_dir, args.pkg_name + "-builded.pkg")
    sh "rm", "-f", *pkgs, stub
end

desc "Check package type of current OS"
task :check_pkg_type do
    puts get_pkg_type()
end

## Agent

desc "Build agent package"
task :build_agent_pkg => [AGENT_PACKAGE_STUB_FILE]

desc "Rebuild agent package"
task :rebuild_agent_pkg do
    sh "rm", "-f", AGENT_PACKAGE_STUB_FILE
    Rake::Task["build_agent_pkg"].invoke()
end

desc "Install agent"
task :install_agent, [:dest] => [agent_dist_dir] do |t, args|
    sh "mkdir", "-p", args.dest
    sh "cp", "-a", "-f", File.join(agent_dist_dir, "."), args.dest
end

## Server & Tool

desc "Build server package"
task :build_server_pkg => [SERVER_PACKAGE_STUB_FILE]

desc "Rebuild server package"
task :rebuild_server_pkg do
    sh "rm", "-f", SERVER_PACKAGE_STUB_FILE
    Rake::Task["build_server_pkg"].invoke()
end

desc "Install server"
task :install_server, [:dest] => [server_dist_dir] do |t, args|
    sh "mkdir", "-p", args.dest
    sh "cp", "-a", "-f", File.join(server_dist_dir, "."), args.dest
end