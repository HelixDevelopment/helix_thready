module thready.integration

go 1.26

require (
	digital.vasic.assetservice v0.0.0
	digital.vasic.callbacktask v0.0.0
	digital.vasic.downloadmanager v0.0.0
	digital.vasic.eventbusservice v0.0.0
	digital.vasic.maxadapter v0.0.0
	digital.vasic.metering v0.0.0
	digital.vasic.metubewebhook v0.0.0
	digital.vasic.ocr v0.0.0
	digital.vasic.restgateway v0.0.0
	digital.vasic.semsearch v0.0.0
	digital.vasic.skilldispatch v0.0.0
	digital.vasic.telegramadapter v0.0.0
	digital.vasic.threadreader v0.0.0
	digital.vasic.userservice v0.0.0
)

// Local resolution for the sibling modules. go.work resolves them for the
// workspace build; these replace directives pin the same local paths so the
// module graph resolves offline (no proxy) for `go work sync` / `go build ./...`
// / `go vet` as well.
replace (
	digital.vasic.assetservice => ../asset_service
	digital.vasic.callbacktask => ../callback_task
	digital.vasic.downloadmanager => ../download_manager
	digital.vasic.eventbusservice => ../event_bus_service
	digital.vasic.maxadapter => ../max_adapter
	digital.vasic.metering => ../metering
	digital.vasic.metubewebhook => ../metube_webhook
	digital.vasic.ocr => ../ocr_adapter
	digital.vasic.restgateway => ../rest_gateway
	digital.vasic.semsearch => ../semantic_search
	digital.vasic.skilldispatch => ../skill_dispatch
	digital.vasic.telegramadapter => ../telegram_adapter
	digital.vasic.threadreader => ../threadreader
	digital.vasic.userservice => ../user_service
)
