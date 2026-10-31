package dynamodb

type AVString struct {
	S string
}

type AVNumber struct {
	N string
}

type AVByte struct {
	B []byte
}

type AVBool struct {
	BOOL bool
}

type AVNull struct {
	NULL bool
}

type AVList struct {
	L []any
}

type AVMap struct {
	M map[string]any
}

type AVStringSet struct {
	SS []string
}

type AVNumberSet struct {
	NS []string
}

type AVByteSet struct {
	BS [][]byte
}
