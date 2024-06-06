package dynamodb

import (
	"errors"
	"log/slog"
)

var (
	ErrNoItems = errors.New("no items found")
)

// Client - dynamodb client to query the table (get,put,query,scan)
type Client struct {
	db     ddbClient
	logger *slog.Logger
	dryRun bool
}

type AVer interface {
	IsAV() bool
}

type AVString struct {
	S string
}

func (s AVString) IsAV() bool {
	return true
}

type AVNumber struct {
	N string
}

func (n AVNumber) IsAV() bool {
	return true
}

type AVByte struct {
	B []byte
}

func (b AVByte) IsAV() bool {
	return true
}

type AVBool struct {
	BOOL bool
}

func (b AVBool) IsAV() bool {
	return true
}

type AVNull struct {
	NULL bool
}

func (n AVNull) IsAV() bool {
	return true
}

type AVList struct {
	L []any
}

func (l AVList) IsAV() bool {
	return true
}

type AVMap struct {
	M map[string]any
}

func (m AVMap) IsAV() bool {
	return true
}

type AVStringSet struct {
	SS []string
}

func (ss AVStringSet) IsAV() bool {
	return true
}

type AVNumberSet struct {
	NS []string
}

func (ns AVNumberSet) IsAV() bool {
	return true
}

type AVByteSet struct {
	BS [][]byte
}

func (bs AVByteSet) IsAV() bool {
	return true
}
