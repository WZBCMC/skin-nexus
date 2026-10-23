package main

// boolStr renders config booleans in the option-value alphabet ("1"/"0") the
// settings panel normalizes to.
func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
