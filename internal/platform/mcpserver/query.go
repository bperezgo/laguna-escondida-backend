package mcpserver

import "net/url"

// query builds a url.Values from key/value pairs, skipping any pair whose value
// is empty. Usage: query("supplier_id", in.SupplierID, "start_date", in.StartDate).
func query(pairs ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			q.Set(pairs[i], pairs[i+1])
		}
	}
	return q
}
