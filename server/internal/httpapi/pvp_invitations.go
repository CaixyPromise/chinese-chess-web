package httpapi

import "net/http"

func (a *API) invitationPreview(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var b struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, err := a.rooms.Preview(owner, b.Code)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, http.StatusOK, v)
}
func (a *API) withdrawInvitation(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var b struct {
		Code          string `json:"code"`
		ApplicationID string `json:"applicationId"`
		RequestID     string `json:"requestId"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, err := a.rooms.WithdrawApplication(owner, b.Code, b.ApplicationID, b.RequestID)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, http.StatusOK, v)
}
