log connection

tcp protocol "gemini" {
    tls keypair foo.zone
    tls keypair snonux.foo
    tls keypair paul.buetow.org
    tls keypair mirror.foo.zone
    tls keypair mirror.snonux.foo
    tls keypair mirror.paul.buetow.org
}

relay "gemini4" {
    listen on <%= $vio0_ip %> port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}

relay "gemini6" {
    listen on <%= $ipv6address->($hostname) %> port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}
