import time
import xmlrpc.client
import subprocess

import pytest

import containers


SUPPORTED_DISTROS = [
    ('ubuntu/18.04', 'centos/7'),
    ('centos/7', 'ubuntu/18.04')
]


def banner(txt):
    print("=" * 80)
    print(txt)


@pytest.mark.parametrize("agent, server", SUPPORTED_DISTROS)
def test_users_management(agent, server):
    """Check if users can be fetched and added."""
    # login
    r = server.api_post('/sessions', json=dict(useremail='admin', userpassword='admin'), expected_status=200)  # TODO: POST should return 201
    assert r.json()['login'] == 'admin'

    # TODO: these are crashing
    # r = server.api_get('/users')
    # r = server.api_post('/users')

    r = server.api_get('/users', params=dict(start=0, limit=10))
    #assert r.json() == {"items":[{"email":"","groups":[1],"id":1,"lastname":"admin","login":"admin","name":"admin"}],"total":1}

    r = server.api_get('/groups', params=dict(start=0, limit=10))
    groups = r.json()
    assert groups['total'] == 2
    assert len(groups['items']) == 2
    assert groups['items'][0]['name'] in ['super-admin', 'admin']
    assert groups['items'][1]['name'] in ['super-admin', 'admin']

    user = dict(
        user=dict(id=0,
                  login='user',
                  email='a@example.org',
                  name='John',
                  lastname='Smith',
                  groups=[]),
        password='password')
    r = server.api_post('/users', json=user, expected_status=200)  # TODO: POST should return 201


@pytest.mark.parametrize("distro_agent, distro_server", SUPPORTED_DISTROS)
def test_pkg_upgrade(distro_agent, distro_server):
    """Check if Stork agent and server can be upgraded from latest release
    to localy built packages."""
    server = containers.StorkServerContainer(alias=distro_server)
    agent = containers.StorkAgentContainer(alias=distro_agent)

    # install the latest version of stork from cloudsmith
    server.setup_bg('cloudsmith')
    agent.setup_bg('cloudsmith')
    server.setup_wait()
    agent.setup_wait()

    # install local packages
    banner('UPGRADING STORK')
    agent.prepare_stork_agent()
    server.prepare_stork_server()

    # install kea on the agent machine
    agent.install_kea()

    # login
    r = server.api_post('/sessions', json=dict(useremail='admin', userpassword='admin'), expected_status=200)  # TODO: POST should return 201
    assert r.json()['login'] == 'admin'

    # add machine and check if it can be retrieved
    machine = dict(
        address=agent.mgmt_ip,
        agentPort=8080)
    r = server.api_post('/machines', json=machine, expected_status=200)  # TODO: POST should return 201
    assert r.json()['address'] == agent.mgmt_ip

    for i in range(100):
        r = server.api_get('/machines')
        data = r.json()
        if len(data['items']) == 1 and data['items'][0]['apps'] and len(data['items'][0]['apps'][0]['details']['daemons']) > 1:
            break
        time.sleep(2)

    m = data['items'][0]
    assert m['apps'] is not None
    assert len(m['apps']) == 1
    assert m['apps'][0]['version'] == '1.7.3'


@pytest.mark.parametrize("agent, server", [('ubuntu/18.04', 'centos/7')])
def test_add_kea_with_many_subnets(agent, server):
    """Check if Stork agent and server will handle Kea instance with huge amount of subnets."""
    # install kea on the agent machine
    agent.install_kea()

    # prepare kea config with many subnets and upload it to the agent
    banner("UPLOAD KEA CONFIG WITH MANY SUBNETS")
    subprocess.run('../../docker/gen-kea-config.py 7000 > kea-dhcp4-many-subnets.conf', shell=True, check=True)
    agent.upload('kea-dhcp4-many-subnets.conf', '/etc/kea/kea-dhcp4.conf')
    subprocess.run('rm -f kea-dhcp4-many-subnets.conf', shell=True)
    agent.run('systemctl restart isc-kea-dhcp4-server')

    # login
    r = server.api_post('/sessions', json=dict(useremail='admin', userpassword='admin'), expected_status=200)  # TODO: POST should return 201
    assert r.json()['login'] == 'admin'

    # add machine
    banner("ADD MACHINE")
    machine = dict(
        address=agent.mgmt_ip,
        agentPort=8080)
    r = server.api_post('/machines', json=machine, expected_status=200)  # TODO: POST should return 201
    assert r.json()['address'] == agent.mgmt_ip

    for i in range(100):
        r = server.api_get('/machines')
        data = r.json()
        if len(data['items']) == 1 and data['items'][0]['apps']:
            break
        time.sleep(2)
    assert len(data['items']) == 1
    m = data['items'][0]
    assert m['apps'] is not None
    assert len(m['apps']) == 1
    assert m['apps'][0]['version'] == '1.7.3'
    assert len(m['apps'][0]['accessPoints']) == 1
    assert m['apps'][0]['accessPoints'][0]['address'] == '127.0.0.1'

    # get subnets (wait until the whole kea config is loaded to stork server)
    # and check if total is huge enough
    banner("GET SUBNETS")
    for i in range(30):
        r = server.api_get('/subnets?start=0&limit=10')
        data = r.json()
        if 'total' in data and data['total'] == 6912:
            break
        time.sleep(2)
    assert data['total'] == 6912


@pytest.mark.parametrize("agent, server", [('centos/7', 'ubuntu/18.04')])
def test_change_kea_ca_access_point(agent, server):
    """Check if Stork server notices that Kea CA has changed its listening address."""
    # install kea on the agent machine
    agent.install_kea()

    # login
    r = server.api_post('/sessions', json=dict(useremail='admin', userpassword='admin'), expected_status=200)  # TODO: POST should return 201
    assert r.json()['login'] == 'admin'

    # add machine
    banner("ADD MACHINE")
    machine = dict(
        address=agent.mgmt_ip,
        agentPort=8080)
    r = server.api_post('/machines', json=machine, expected_status=200)  # TODO: POST should return 201
    assert r.json()['address'] == agent.mgmt_ip

    for i in range(40):
        r = server.api_get('/machines')
        data = r.json()
        if len(data['items']) == 1 and data['items'][0]['apps']:
            break
        time.sleep(2)
    assert len(data['items']) == 1
    m = data['items'][0]
    assert m['apps'] is not None
    assert len(m['apps']) == 1
    assert m['apps'][0]['version'] == '1.7.3'
    assert len(m['apps'][0]['accessPoints']) == 1
    assert m['apps'][0]['accessPoints'][0]['address'] == '127.0.0.1'

    # stop and reconfigure CA to serve from different IP address
    banner("STOP CA and reconfigure listen IP address")
    agent.run('sed -i -e s/"0.0.0.0"/"%s"/g /etc/kea/kea-ctrl-agent.conf' % agent.mgmt_ip)
    ca_svc_name = 'kea-ctrl-agent' if 'centos' in agent.name else 'isc-kea-ctrl-agent'
    agent.run('systemctl stop ' + ca_svc_name)

    # wait for unreachable event
    banner("WAIT FOR UNREACHABLE EVENT")
    event_occured = False
    for i in range(20):
        r = server.api_get('/events')
        data = r.json()
        if 'is unreachable' in data['items'][0]['text']:
            event_occured = True
            break
        time.sleep(2)
    assert event_occured, 'no event about unreachable kea'

    # start CA
    banner("START CA")
    agent.run('systemctl start ' + ca_svc_name)

    # wait for reachable event
    banner("WAIT FOR REACHABLE EVENT")
    event_occured = False
    for i in range(20):
        r = server.api_get('/events')
        data = r.json()
        if 'is reachable now' in data['items'][0]['text']:
            event_occured = True
            break
        time.sleep(2)
    assert event_occured, 'no event about reachable kea'

    # check for sure if app has new access point address
    banner("CHECK IF RECONFIGURED")
    r = server.api_get('/machines')
    data = r.json()
    assert len(data['items']) == 1
    m = data['items'][0]
    assert m['apps'] is not None
    assert len(m['apps']) == 1
    assert len(m['apps'][0]['accessPoints']) == 1
    assert m['apps'][0]['accessPoints'][0]['address'] == agent.mgmt_ip
