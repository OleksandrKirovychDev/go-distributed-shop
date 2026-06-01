// The router validates the edge JWT (authentication.router.jwt) and stores the
// verified claims in the request context. Forward the principal to subgraphs as
// the headers the inbound GraphQL adapter trusts (x-user-id / x-user-roles),
// matching the metadata keys the gRPC adapter reads. Anonymous requests (no token,
// e.g. registerUser/loginUser) carry no claims and pass through header-free, so the
// subgraph treats them as unauthenticated.
//
// A closure (not a named fn reference) is registered so the callback runs without
// locking the global script scope on every subgraph request.
fn subgraph_service(service, subgraph) {
    service.map_request(|request| {
        let claims = request.context[Router.APOLLO_AUTHENTICATION_JWT_CLAIMS];
        if claims != () {
            let user_id = claims["sub"];
            if user_id != () {
                request.subgraph.headers["x-user-id"] = user_id;
            }

            let roles = claims["roles"];
            if roles != () {
                let roles_csv = "";
                for role in roles {
                    if roles_csv != "" {
                        roles_csv += ",";
                    }
                    roles_csv += role;
                }
                if roles_csv != "" {
                    request.subgraph.headers["x-user-roles"] = roles_csv;
                }
            }
        }
    });
}
