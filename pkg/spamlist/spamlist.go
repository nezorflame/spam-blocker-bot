package spamlist

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// SpamList is used for spam protection
type SpamList struct {
	UserIDs []int     `json:"user_ids"`
	Date    time.Time `json:"date"`

	cfg *viper.Viper
	mtx sync.RWMutex
}

// New creates new SpamList, importing it from CAS API
func New(cfg *viper.Viper) (list *SpamList) {
	var err error
	list = &SpamList{Date: time.Now(), cfg: cfg}
	// defer list loading from filesystem if we had an error in the process
	defer func() {
		if err != nil {
			if err = list.Load(); err != nil {
				log.WithError(err).Warn("Unable to load CAS list from filesystem")
			}
		}
	}()

	resp, err := http.Get(list.cfg.GetString("cas.export_url"))
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

	check, err := lookup(sl.cfg.GetString("cas.check_url"), id)
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

// Save saves spam list to the filesystem as JSON
func (sl *SpamList) Save() error {
	sl.mtx.RLock()
	defer sl.mtx.RUnlock()

	data, err := json.Marshal(sl)
	if err != nil {
		return errors.Wrap(err, "unable to marshal spamlist")
	}

	path := filepath.Clean(sl.cfg.GetString("cas.local_file"))
	if err = ioutil.WriteFile(path, data, os.ModePerm); err != nil {
		return errors.Wrap(err, "unable to save spamlist")
	}

	return nil
}

// Load loads JSON spam list from the filesystem
func (sl *SpamList) Load() error {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()

	data, err := ioutil.ReadFile(filepath.Clean(sl.cfg.GetString("cas.local_file")))
	if err != nil {
		return errors.Wrap(err, "unable to load spamlist file")
	}

	if err = json.Unmarshal(data, sl); err != nil {
		return errors.Wrap(err, "unable to unmarshal spamlist")
	}

	return nil
}

func lookup(url string, id int) (bool, error) {
	resp, err := http.Get(fmt.Sprintf(url, id))
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

	var result, ok bool
	if result, ok = respMap["ok"].(bool); !ok {
		return false, errors.New("check result is not a bool")
	}

	return result, nil
}
