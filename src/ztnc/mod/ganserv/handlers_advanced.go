package ganserv

import (
	"encoding/json"
	"net/http"

	"aroz.org/zoraxy/ztnc/mod/utils"
)

func (m *NetworkManager) HandleAdvancedUpdate(w http.ResponseWriter, r *http.Request) {
	netid, err := utils.PostPara(r, "netid")
	if err != nil || netid == "" {
		utils.SendErrorResponse(w, "netid not set")
		return
	}
	var s AdvancedNetworkSettings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		utils.SendErrorResponse(w, "invalid json body")
		return
	}
	if err := m.ApplyAdvancedSettings(netid, &s); err != nil {
		utils.SendErrorResponse(w, err.Error())
		return
	}
	utils.SendOK(w)
}

func (m *NetworkManager) HandleSetControllerBridge(w http.ResponseWriter, r *http.Request) {
	netid, err := utils.PostPara(r, "netid")
	if err != nil || netid == "" {
		utils.SendErrorResponse(w, "netid not set")
		return
	}
	uplink, _ := utils.PostPara(r, "uplink")    // e.g. eth0
	ztIfHint, _ := utils.PostPara(r, "ztif")    // optional override, else derive
	hostOnly, _ := utils.PostPara(r, "hostOnly") // "true" to only prep sysctl without enslaving uplink

	// Mark controller member as bridge on the network
	if err := m.SetMemberBridge(netid, m.ControllerID, true); err != nil {
		utils.SendErrorResponse(w, "controller bridge flag: "+err.Error())
		return
	}
	// Kick off host bridge
	if err := InvokeHostBridge(netid, uplink, ztIfHint, hostOnly == "true"); err != nil {
		utils.SendErrorResponse(w, "host bridge: "+err.Error())
		return
	}
	utils.SendOK(w)
}
