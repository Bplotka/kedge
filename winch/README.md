# Winch

![winch](winch.jpg)

Forward proxy for gRPC, HTTP (1.1/2) microservices used as a local proxy to the clusters with the kedge at the edges.
This allows to have safe route to the internal services by the authorized user.

## Usage
1. Specify rules for routing to proper kedges.
2. Run application on you local machine:

    ```
    go run ./winch/server/*.go \
      --server_http_port=8098 \
      --server_config_mapper_path=./misc/winch_mapper.json
    ```
3. Forward traffic to the `http://127.0.0.1:8098`

### Forwarding from browser

TBD: PAC file.

### Forwarding from CLI 

To force an application to dial required URL through winch just set `HTTP_PROXY` environment variable to the winch localhost address.
 
## Status

* [ ] - forward Proxy to remote Kedges for a CLI command (setting HTTP_PROXY) "kedge_local <cmd>"
    * [x] - HTTP
    * [ ] - gRPC
    * [ ] - HTTPS
* [ ] - forward Proxy in daemon mode with an auto-gen [PAC](https://en.wikipedia.org/wiki/Proxy_auto-config) file
    * [ ] - HTTP
    * [ ] - gRPC
    * [ ] - HTTPS
* [x] - matching logic for "remap something.my_cluster.cluster.local to my_cluster.internalapi.example.com" for finding Kedges on the internet
* [ ] - support for custom root CA for TLS with kedge
* [ ] - reading of TLS client certs from ~/.config/kedge
* [ ] - open ID connect login to get ID token / refresh token


