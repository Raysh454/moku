package enumerator

type Enumerator interface {
	Enumerate(target string) ([]string, error)
}
