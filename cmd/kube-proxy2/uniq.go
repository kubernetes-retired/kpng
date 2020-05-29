package main

type uniq []string

func (u *uniq) Add(v string) {
	for _, s := range *u {
		if s == v {
			return
		}
	}

	*u = append(*u, v)
}
