package counter

type Counter interface {
	CountUniqueIPs(filePath string) (uint64, error)
}
