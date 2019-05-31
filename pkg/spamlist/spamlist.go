package spamlist

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	combotCheckURL  = "https://combot.org/api/cas/check?user_id=%d"
	combotExportURL = "https://combot.org/api/cas/export.csv"
)

// SpamList is used for spam protection
type SpamList struct {
	UserIDs []int
	Date    time.Time

	mtx sync.RWMutex
}

// New creates new SpamList, importing it from CAS API
func New() (list *SpamList) {
	list = &SpamList{Date: time.Now()}

	resp, err := http.Get(combotExportURL)
	if err != nil {
		log.WithError(err).Warn("Unable to import CAS list")
		return
	}
	defer resp.Body.Close()

	csvReader := csv.NewReader(resp.Body)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.WithError(err).Warn("Unable to read CAS list")
		return
	}

	list.UserIDs = make([]int, 0, len(records))
	for _, r := range records {
		if len(r) == 0 {
			continue
		}

		var id int
		if id, err = strconv.Atoi(r[0]); err != nil {
			continue
		}

		list.UserIDs = append(list.UserIDs, id)
	}

	return
}

// CheckUser looks up the user in the spam list
func (sl *SpamList) CheckUser(id int) (check bool, ok bool) {
	sl.mtx.RLock()
	defer sl.mtx.RUnlock()

	for _, uid := range sl.UserIDs {
		if uid == id {
			return true, true
		}
	}

	check, err := lookup(id)
	if err != nil {
		log.WithError(err).Warnf("Unable to check user '%d'", id)
		return false, false
	}

	return check, false
}

// Add adds new user to the spam list
func (sl *SpamList) Add(id int) {
	sl.mtx.Lock()
	sl.UserIDs = append(sl.UserIDs, id)
	sl.mtx.Unlock()
}

func lookup(id int) (bool, error) {
	resp, err := http.Get(fmt.Sprintf(combotCheckURL, id))
	if err != nil {
		return false, errors.Wrap(err, "unable to call CAS API")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, errors.Wrap(err, "unable to read check result")
	}

	respMap := make(map[string]interface{})
	if err = json.Unmarshal(body, &respMap); err != nil {
		return false, errors.Wrap(err, "unable to parse check result")
	}
	log.Debug(respMap)

	var result, ok bool
	if result, ok = respMap["ok"].(bool); !ok {
		log.Debugf("Bad or empty check result: '%v'", respMap["ok"])
		return false, errors.New("check result is not a bool")
	}

	return result, nil
}
