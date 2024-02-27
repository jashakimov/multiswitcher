package utils

func InSlice(need string, arr []string) bool {
	for i := range arr {
		if arr[i] == need {
			return true
		}
	}
	return false
}
