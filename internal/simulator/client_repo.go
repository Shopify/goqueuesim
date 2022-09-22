package simulator

import (
	"sync"

	"github.com/Shopify/goqueuesim/internal/client"
)

type ClientRepo interface {
	WriteClient(c client.Client)
	FetchClientById(id int) client.Client
}

type SimpleClientRepo struct {
	sync.Mutex
	dict map[int]client.Client
}

func MakeSimpleClientRepo(d map[int]client.Client) *SimpleClientRepo {
	return &SimpleClientRepo{dict: d}
}

func (r *SimpleClientRepo) WriteClient(c client.Client) {
	r.Lock()
	r.dict[c.ID()] = c
	r.Unlock()
}

func (r *SimpleClientRepo) FetchClientById(id int) client.Client {
	r.Lock()
	c := r.dict[id]
	r.Unlock()
	return c
}
