package console

const (
	instanceURL             = "https://console.iwinv.kr/instance"
	createServiceURL        = "https://console.iwinv.kr/instance/request/service"
	firewallURL             = "https://console.iwinv.kr/firewall"
	firewallTabURL          = "https://console.iwinv.kr/firewall/tab"
	supportRequestAgreeJSON = "https://console.iwinv.kr/js/console/support/request/agree.json"
	supportRequestWrite     = "https://console.iwinv.kr/support/request/write"
	optionURLPattern        = "**/instance/request/option"
	confirmURLPattern       = "**/instance/request/confirm"
	successURLPattern       = "**/instance/request/success"
	firewallTable2XPath     = `/html/body/div[2]/div[2]/div/main/section/div/div[2]/div/table[2]`

	popupCloseXPath    = `/html/body/div[2]/div[2]/div/main/div[2]/div/div[1]/button[1]`
	region1XPath       = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[1]/div/div[2]/div[1]`
	region2XPath       = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[1]/div/div[2]/div[2]`
	specXPath          = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[2]/div/div[2]/div/table[2]/tbody`
	osTabDefaultXPath  = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[3]/div/div[2]/div[1]/div[1]`
	osTabMyXPath       = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[3]/div/div[2]/div[1]/div[2]`
	osTabSolutionXPath = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[3]/div/div[2]/div[1]/div[3]`
	osTableXPath       = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[3]/div/div[2]/div[3]//table/tbody`

	blockTypeXPath = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[4]/div/div[2]/div[1]/div[1]/div[1]/div/el-select/button/el-selectedcontent`
	blockSizeXPath = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[4]/div/div[2]/div[1]/div[1]/div[2]/div/div/div/input`
	blockNameXPath = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[4]/div/div[2]/div[1]/div[1]/div[3]/div/input`
	blockAddXPath  = `/html/body/div[2]/div[2]/div/main/form/div[2]/div/section[4]/div/div[2]/div[1]/div[1]/a`

	createNextXPath    = `/html/body/div[2]/div[2]/div/main/form/div[4]/div/a`
	serverNameXPath    = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[1]/div/section[1]/div/div[2]/dd/input`
	qtyXPath           = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[1]/div/section[2]/div/div[2]/dd/div/input`
	multiPasswordXPath = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[1]/div/section[2]/div/div[3]/dd/input`

	firewallEnableXPath = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[1]/div/section[3]/div/div[2]/dd/div/label[1]/div[1]/input`
	firewallListXPath   = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[1]/div/section[3]/div/div[3]/div[1]/dd`
	firewallNextXPath   = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[1]/div/section[3]/div/div[3]/div[2]/nav/div/button`
	optionConfirmXPath  = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[2]/div[2]/div/div/a[2]`
	summaryXPath        = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[1]/section/div`
	finalCreateXPath    = `/html/body/div[2]/div[2]/div/main/form/div[2]/div[2]/div/div/a[2]`
)
