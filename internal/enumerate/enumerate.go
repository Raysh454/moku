package enumerate

type Enumerate interface {
	Enumerate(target string) ([]string, error)
}
