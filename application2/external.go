package application2

import "uuid"

type External interface {
	id() uuid.UUID
	getValue(key string) any
}
