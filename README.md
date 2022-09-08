## mixctl 🍸 - a tiny TCP load balancer 

mixctl by [inlets](https://docs.inlets.dev) is a tiny TCP load balancer. It was written to help [inlets users](https://docs.inlets.dev) to expose multiple services hosted on different servers over a single TCP tunnel.

## What's it for?

mixctl can be used to replace HAProxy, Traefik and/or Nginx with streams in certain scenarios.

It could also be used as a lightweight load-balancer for K3s servers.

Perhaps use this instead of IP Virtual Server (IPVS)?

This is a lightweight, multi-arch, multi-OS, uncomplicated way to reverse proxy different TCP connections and/or load balance them.

[IPVS](https://debugged.it/blog/ipvs-the-linux-load-balancer/) is going to be more performant, but requires a Linux host and capabilities, which you may be remiss to grant through Docker or Kubernetes.

## Usage:

1) Write a rules.yaml file such as: [./rules.example.yaml](./rules/example.yaml):

```yaml
version: 0.1

rules:
- name: rpi-k3s
  from: 127.0.0.1:6443
  to:
    - 192.168.1.19:6443
    - 192.168.1.21:6443
    - 192.168.1.20:6443

- name: rpi-ssh
  from: 127.0.0.1:22222
  to:
    - 192.168.1.19:22
    - 192.168.1.21:22
    - 192.168.1.20:22
```

2) Run the tool: `mixctl -f ./rules.yaml`

3) Run `inlets-pro tcp client --ports 6443 --ports 22222 --upstream 127.0.0.1`

4) Connect to ports 6443 or 22222 on your inlets Pro tunnel server to access any of the servers in the "to" array

Connections are load balanced if there are multiple hosts in the `to` field.

To make the upstream address listen on all interfaces, use `0.0.0.0` instead of `127.0.0.1` in the `from` field.

The port for the from and to addresses do not need to match.

## License

This software is licensed MIT.
