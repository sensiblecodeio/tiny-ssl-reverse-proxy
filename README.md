A tiny SSL reverse proxy
========================

Did you ever want to protect your docker container with SSL, but
didn't want to have to pull the whole of nginx? Well now you can!

Usage:
------

```
tiny-ssl-reverse-proxy [-key /etc/ssl/private/key.pem]
                       [-cert /etc/ssl/private/cert.pem]
                       [-listen :443] [-htpasswdFile htpasswd]
                       -where http://forward:8080/

Options:
    -key            Path to host private key
    -cert           Path to host certificate
    -listen         Bind address
    -htpasswdFile   Path to htpasswd file
    -where          Target host to forward to
```
