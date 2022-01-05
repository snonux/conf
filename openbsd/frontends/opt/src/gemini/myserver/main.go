package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/a-h/gemini"
)

type configuration struct {
	// Domain name, e.g. localhost.
	domain string
	// Certfile is the path to a server cerfificate file.
	certFile string
	// Keyfile is the path to a server key file.
	keyFile string
	// Path to Gemini content to serve.
	path string
}

func main() {
	config := []configuration{
		{
			domain:   "buetow.org",
			certFile: "/etc/ssl/buetow.org.fullchain.pem",
			keyFile:  "/etc/ssl/private/buetow.org.key",
			path:     "/var/gemini/gemtexter/buetow.org",
		},
		{
			domain:   "snonux.de",
			certFile: "/etc/ssl/snonux.de.fullchain.pem",
			keyFile:  "/etc/ssl/private/snonux.de.key",
			path:     "/var/gemini/gemtexter/snonux.de",
		},
	}

	// Load the config.
	domainToHandler := map[string]*gemini.DomainHandler{}

	for i := 0; i < len(config); i++ {
		c := config[i]
		h := gemini.FileSystemHandler(gemini.Dir(c.path))
		cert, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
		if err != nil {
			fmt.Printf("error: failed to load certificates for domain %q: %v\n", c.domain, err)
			os.Exit(1)
		}
		dh := gemini.NewDomainHandler(c.domain, cert, h)
		domainToHandler[c.domain] = dh
	}

	// Start the server.
	ctx := context.Background()
	server := gemini.NewServer(ctx, ":1965", domainToHandler)
	server.ReadTimeout = time.Second * 5
	server.WriteTimeout = time.Second * 10
	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
