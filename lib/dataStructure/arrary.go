package ds

import (
	"encoding/json"
	"sync"
)

type IDerArrary struct {
	*sync.RWMutex
	list    []IDer
	idIndex map[interface{}]int
}

type IDer interface {
	ID() interface{}
}

func NewIDerArrary() *IDerArrary {
	return &IDerArrary{
		RWMutex: new(sync.RWMutex),
		list:    make([]IDer, 0),
		idIndex: make(map[interface{}]int),
	}
}

func (a *IDerArrary) ListMarshal() []byte {
	a.RLock()
	defer a.RUnlock()
	data, _ := json.Marshal(a.list)
	return data
}

func (a *IDerArrary) Append(i IDer) {
	a.Lock()
	a.list = append(a.list, i)
	a.idIndex[i.ID()] = len(a.list) - 1
	a.Unlock()
}

func (a *IDerArrary) Remove(id interface{}) {
	a.Lock()
	defer a.Unlock()
	idx, ok := a.idIndex[id]
	if !ok {
		return
	}
	copy(a.list[idx:], a.list[idx+1:])
	a.list = a.list[:len(a.list)-1]
	delete(a.idIndex, id)
	for k, v := range a.idIndex {
		if v > idx {
			a.idIndex[k] = v - 1
		}
	}

}

func (a *IDerArrary) Has(id interface{}) bool {
	a.RLock()
	defer a.RUnlock()
	_, ok := a.idIndex[id]
	return ok
}
