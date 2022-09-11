## mixctl 🍸 - a tiny TCP load balancer 

mixctl by [inlets](https://docs.inlets.dev) is a tiny TCP load balancer written in Go. It was created to help [inlets users](https://docs.inlets.dev) to expose multiple services hosted on different servers over a single TCP tunnel.

## What's it for?

mixctl can be used to replace HAProxy, Traefik and/or Nginx Streams in certain scenarios. It could also be used as a lightweight load-balancer for K3s servers.

This is a lightweight, multi-arch, multi-OS, uncomplicated way to reverse proxy different TCP connections and/or load balance them.

## Usage:

1) Write a `rules.yaml` file such as: [rules.example.yaml](rules.example.yaml):

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

3) Test out the proxy by calling your local endpoint such as `curl -k https://127.0.0.1:6443` and you should get a response back from each of the upstream endpoints.

4) Now, if you're an inlets user, run `inlets-pro tcp client --ports 6443 --ports 22222 --upstream 127.0.0.1`, this exposes the ports that mixctl is listening to the tunnel server.

4) Connect to ports 6443 or 22222 on your inlets Pro tunnel server to access any of the servers in the "to" array. The connections will be load balanced (with a random spread) if there are multiple hosts in the `to` field.

To make the upstream address listen on all interfaces, use `0.0.0.0` instead of `127.0.0.1` in the `from` field.

The port for the from and to addresses do not need to match.

See also:
* `-t` - specify the dial timeout for an upstream host in the "to" field of the config file.
* `-v` - verbose logging - set to false to turn off logs of connections established and closed.

## License

This software is licensed MIT.

## See also

### inlets-connect

[inlets-connect](https://github.com/alexellis/inlets-connect) is an equally tiny HTTP CONNECT proxy, designed to help users proxy HTTP and HTTPS endpoints over a single inlets tunnel.

### "cloud-provision" library

The [cloud-provision](https://github.com/inlets/cloud-provision) library is used by [inletsctl](https://github.com/inlets/inletsctl) to create HTTP and TCP tunnel servers (VMs) with inlets pre-installed.

You can think of it like a low-level Terraform, which supports various popular clouds and VPS providers. Specify the plan, name, and user-data to configure the node with your desired software.

### IPVS as an alternative to mixctl

IP Virtual Server (IPVS)? [IPVS](https://debugged.it/blog/ipvs-the-linux-load-balancer/) is going to be more performant because it's part of the Linux kernel-space, instead of user-space (where normal programs like mixctl run). However IPVS requires a Linux host, additional Kernel modules to be loaded, and special Linux Kernel privileges, which you may be remiss to grant if using Docker or Kubernetes.

Instead, mixctl is a lightweight, multi-arch, multi-OS, uncomplicated way to reverse proxy different TCP connections and/or load balance them.

