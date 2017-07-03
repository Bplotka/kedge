# Winch

![winch](winch.jpg)

Local proxy for gRPC, HTTP (1.1/2) microservices used as a router to the clusters with the kedges at the edge.
This allows to have safe route to the internal services by the authorized user.

## Usage

### For browser

TBD: PAC file.

### For application 

To force an application to dial required URL through winch just set `HTTP_PROXY` environment variable to the winch localhost address.
 
## Status

* [] Open ID connect login to get ID token / refresh token
* [] HTTP local proxy based on routes
* [] gRPC local proxy based on routes
