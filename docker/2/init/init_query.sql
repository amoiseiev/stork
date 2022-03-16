delete from lease4;
insert into lease4(address, hwaddr, client_id, valid_lifetime, expire, subnet_id, fqdn_fwd, fqdn_rev, hostname, state) values (INET_ATON('192.0.2.1'), UNHEX('1f1e1f1e1f1e'), UNHEX('61626162'), 3600, DATE_ADD(NOW(), INTERVAL 1 MONTH), 1, false, false, 'client-1.example.org', 0);
insert into lease4(address, hwaddr, valid_lifetime, expire, subnet_id, hostname, state) values (INET_ATON('192.0.2.2'), '', 3600, DATE_ADD(NOW(), INTERVAL 1 MONTH), 1, '', 1);