package main

import (
	"github.com/chubaofs/chubaofs/console/service"
	"github.com/chubaofs/chubaofs/sdk/master"
	"log"
	"net/http"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/example/starwars"
	"github.com/graph-gophers/graphql-go/relay"
)

func main() {

	add_handle("demo", starwars.Schema, &starwars.Resolver{}, true)

	mc := master.NewMasterClientFromString("", false)

	userService := &service.UserService{UserApi: mc.UserAPI()}
	add_handle("user", userService.Schema(), userService, true)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func add_handle(model string, schema string, service interface{}, iql bool) {
	if iql {
		http.Handle("/"+model, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(graphiql(model))
		}))
	}

	http.Handle("/"+model+"/", &relay.Handler{Schema: graphql.MustParseSchema(schema, service, graphql.UseStringDescriptions(), graphql.UseFieldResolvers())})
}

func graphiql(model string) []byte {
	return []byte(`
<!DOCTYPE html>
<html>
	<head>
		<link href="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.11.11/graphiql.min.css" rel="stylesheet" />
		<script src="https://cdnjs.cloudflare.com/ajax/libs/es6-promise/4.1.1/es6-promise.auto.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/fetch/2.0.3/fetch.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react/16.2.0/umd/react.production.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react-dom/16.2.0/umd/react-dom.production.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.11.11/graphiql.min.js"></script>
	</head>
	<body style="width: 100%; height: 100%; margin: 0; overflow: hidden;">
		<div id="graphiql" style="height: 100vh;">Loading...</div>
		<script>
			function graphQLFetcher(graphQLParams) {
				return fetch("/` + model + `/", {
					method: "post",
					body: JSON.stringify(graphQLParams),
					credentials: "include",
				}).then(function (response) {
					return response.text();
				}).then(function (responseBody) {
					try {
						return JSON.parse(responseBody);
					} catch (error) {
						return responseBody;
					}
				});
			}
			ReactDOM.render(
				React.createElement(GraphiQL, {fetcher: graphQLFetcher}),
				document.getElementById("graphiql")
			);
		</script>
	</body>
</html>
`)
}
