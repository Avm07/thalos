package message

import "encoding/json"

type HeartBeat struct {
	BlockNum                 uint32 `json:"blocknum" msgpack:"blocknum"`
	HeadBlockNum             uint32 `json:"head_blocknum" msgpack:"head_blocknum"`
	LastIrreversibleBlockNum uint32 `json:"last_irreversible_blocknum" msgpack:"last_irreversible_blocknum"`
}

type PermissionLevel struct {
	Actor      string `json:"actor" msgpack:"actor"`
	Permission string `json:"permission" msgpack:"permission"`
}

type ActionTrace struct {
	TxID string `json:"tx_id" msgpack:"tx_id"`

	// Action name
	Name string `json:"name" msgpack:"name"`

	// Contract account.
	Contract string `json:"contract" msgpack:"contract"`

	Receiver string `json:"receiver" msgpack:"receiver"`
	Data     []byte `json:"data" msgpack:"data"`
	HexData  string `json:"hex_data" msgpack:"hex_data"`

	Authorization []PermissionLevel `json:"authorization" msgpack:"authorization"`

	Except string `json:"except" msgpack:"except"`
	Error  uint64 `json:"error" msgpack:"error"`
	Return []byte `json:"return" msgpack:"return"`
}

func (act ActionTrace) GetData() (map[string]interface{}, error) {
	data := map[string]interface{}{}
	return data, json.Unmarshal(act.Data, &data)
}
