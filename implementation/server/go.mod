module thready.server

go 1.26

require (
	digital.vasic.eventbusservice v0.0.0
	digital.vasic.restgateway v0.0.0
	digital.vasic.semsearch v0.0.0
	digital.vasic.skilldispatch v0.0.0
	digital.vasic.userservice v0.0.0
)

// Local resolution for the sibling modules. go.work resolves them for the
// workspace build; these replace directives pin the same local paths so the
// module graph resolves offline (no proxy) for `go build ./...` / `go vet` /
// `go test` as well. Mirrors implementation/integration/go.mod's pattern.
replace (
	digital.vasic.eventbusservice => ../event_bus_service
	digital.vasic.restgateway => ../rest_gateway
	digital.vasic.semsearch => ../semantic_search
	digital.vasic.skilldispatch => ../skill_dispatch
	digital.vasic.userservice => ../user_service
)
