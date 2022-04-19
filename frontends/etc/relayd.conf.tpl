log connection

tcp protocol "gemini" {
    tls keypair buetow.org
    tls keypair snonux.de
    tls keypair foo.zone
    tls keypair irregular.ninja
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
