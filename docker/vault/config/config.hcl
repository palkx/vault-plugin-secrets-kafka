ui = true
disable_mlock = true

listener "tcp" {
  address = "[::]:8200"
  cluster_address = "[::]:8201"
  tls_disable = 1
}

storage "file" {
  path = "/var/lib/vault"
}

