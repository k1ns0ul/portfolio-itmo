package workers

import "encoding/json"

func jsonDecode(raw []byte, out any) error { return json.Unmarshal(raw, out) }
