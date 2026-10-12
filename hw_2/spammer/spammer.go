package main

import (
	"fmt"
	"sort"
	"sync"
)

func RunPipeline(cmds ...cmd) {
	var wg sync.WaitGroup
	channels := make([]chan interface{}, len(cmds)+1)
	for i := 0; i < len(channels); i++ {
		if i == 0 || i == len(channels)-1 {
			continue
		}
		channels[i] = make(chan interface{})
	}
	for i := range cmds {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmds[i](channels[i], channels[i+1])
			if channels[i+1] != nil {
				close(channels[i+1])
			}
		}(i)
	}
	wg.Wait()
}

func SelectUsers(in, out chan interface{}) {
	// 	in - string
	// 	out - User
	uniqUsers := make(map[uint64]struct{})
	result := make(chan User)
	var wg sync.WaitGroup
	go func() {
		for val := range in {
			wg.Add(1)
			go func(val interface{}) {
				defer wg.Done()
				result <- GetUser(val.(string))
			}(val)
		}
		wg.Wait()
		close(result)
	}()
	for user := range result {
		if _, ok := uniqUsers[user.ID]; ok {
			continue
		}
		uniqUsers[user.ID] = struct{}{}
		out <- user
	}
}

func SelectMessages(in, out chan interface{}) {
	// 	in - User
	// 	out - MsgID
	batch := make([]User, 0, GetMessagesMaxUsersBatch)
	var wg sync.WaitGroup

	for us := range in {
		batch = append(batch, us.(User))
		if len(batch) == GetMessagesMaxUsersBatch {
			wg.Add(1)
			go func(batch []User) {
				defer wg.Done()
				res, err := GetMessages(batch...)
				if err != nil {
					fmt.Printf("error occured: %v\nskipping batch\n", err)
				} else {
					returnSeq(res, out)
				}
			}(batch)
			batch = make([]User, 0, GetMessagesMaxUsersBatch)
		}

	}
	if len(batch) != 0 {
		wg.Add(1)
		go func(batch []User) {
			defer wg.Done()
			res, err := GetMessages(batch...)
			if err != nil {
				fmt.Printf("error occured: %v\nskipping batch\n", err)
			} else {
				returnSeq(res, out)
			}
		}(batch)
	}
	wg.Wait()
}

func returnSeq(seq []MsgID, out chan interface{}) {
	for _, entry := range seq {
		out <- entry
	}
}

func CheckSpam(in, out chan interface{}) {
	// in - MsgID
	// out - MsgData
	var wg sync.WaitGroup
	queue := make(chan struct{}, HasSpamMaxAsyncRequests)
	for entry := range in {
		wg.Add(1)
		go func(entry interface{}) {
			defer func() {
				<-queue
				wg.Done()
			}()
			queue <- struct{}{}
			res, err := HasSpam(entry.(MsgID))
			if err != nil {
				fmt.Printf("error occured: %v\nskipping message\n", err)
			} else {
				out <- MsgData{ID: entry.(MsgID), HasSpam: res}
			}
		}(entry)
	}
	wg.Wait()

}

func CombineResults(in, out chan interface{}) {
	// in - MsgData
	// out - string
	slice := make([]MsgData, 0)
	for val := range in {
		slice = append(slice, val.(MsgData))
	}
	sort.Slice(slice, func(i, j int) bool {
		if slice[i].HasSpam != slice[j].HasSpam {
			return slice[i].HasSpam
		}

		return slice[i].ID < slice[j].ID
	})
	for _, val := range slice {
		out <- fmt.Sprintf("%t %d", val.HasSpam, val.ID)
	}

}
