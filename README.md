A tiny SSL reverse proxy
========================

Did you ever want to protect your docker container with SSL, but
didn't want to have to pull the whole of nginx? Well now you can!

Usage:
------

```
tiny-ssl-reverse-proxy

Usage of tiny-ssl-reverse-proxy:
  -behind-tcp-proxy
    	running behind TCP proxy (such as ELB or HAProxy)
  -cert string
    	Path to PEM certificate (default "/etc/ssl/private/cert.pem")
  -flush-interval duration
    	minimum duration between flushes to the client (default: off)
  -key string
    	Path to PEM key (default "/etc/ssl/private/key.pem")
  -listen string
    	Bind address to listen on (default ":443")
  -logging
    	log requests (default true)
  -tls
    	accept HTTPS connections (default true)
  -where string
    	Place to forward connections to (default "http://localhost:80")
```
