package models

// TODO(Uliana): generate this model in flow repo
type ExecuteScriptResponse struct {
	Value    []byte    `json:"value"`
	Metadata *Metadata `json:"metadata,omitempty"`
}
