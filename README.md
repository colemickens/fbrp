fbrp
====

This is FBRP: "FaceBook Reverse Proxy".

This started out with the idea of making a generic reverse proxy protected by Facebook auth.

For now, it's more of a simple file server (powered by net/http's FileServer) and protected by membership in a secret Facebook group.

It expects users to login via Facebook and only allows users who are members of a specific, secret group.

There's a systemd unit file for this as well.

