package interfaces

type Enumerator interface {
	Enumerate(target string) ([]string, error)
}
