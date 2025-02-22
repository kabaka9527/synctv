package op

import (
	"errors"
	"hash/crc32"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/synctv-org/synctv/internal/db"
	"github.com/synctv-org/synctv/internal/model"
	"github.com/synctv-org/synctv/utils"
	rtmps "github.com/zijiren233/livelib/server"
	"github.com/zijiren233/stream"
	"golang.org/x/crypto/bcrypt"
)

type Room struct {
	model.Room
	version  uint32
	current  *current
	initOnce utils.Once
	hub      *Hub
	movies   movies
}

func (r *Room) lazyInitHub() {
	r.initOnce.Do(func() {
		r.hub = newHub(r.ID)
	})
}

func (r *Room) ClientNum() int64 {
	if r.hub == nil {
		return 0
	}
	return r.hub.ClientNum()
}

func (r *Room) Broadcast(data Message, conf ...BroadcastConf) error {
	if r.hub == nil {
		return nil
	}
	return r.hub.Broadcast(data, conf...)
}

func (r *Room) SendToUser(user *User, data Message) error {
	if r.hub == nil {
		return nil
	}
	return r.hub.SendToUser(user.ID, data)
}

func (r *Room) GetChannel(channelName string) (*rtmps.Channel, error) {
	return r.movies.GetChannel(channelName)
}

func (r *Room) close() {
	if r.initOnce.Done() {
		r.hub.Close()
		r.movies.Close()
	}
}

func (r *Room) Version() uint32 {
	return atomic.LoadUint32(&r.version)
}

func (r *Room) CheckVersion(version uint32) bool {
	return atomic.LoadUint32(&r.version) == version
}

func (r *Room) UpdateMovie(movieId string, movie model.BaseMovie) error {
	return r.movies.Update(movieId, movie)
}

func (r *Room) AddMovie(m model.Movie) error {
	m.RoomID = r.ID
	return r.movies.Add(&m)
}

func (r *Room) HasPermission(userID string, permission model.Permission) bool {
	ur, err := db.GetRoomUserRelation(r.ID, userID)
	if err != nil {
		return false
	}
	return ur.HasPermission(permission)
}

func (r *Room) NeedPassword() bool {
	return len(r.HashedPassword) != 0
}

func (r *Room) SetPassword(password string) error {
	if r.CheckPassword(password) && r.NeedPassword() {
		return errors.New("password is the same")
	}
	var hashedPassword []byte
	if password != "" {
		var err error
		hashedPassword, err = bcrypt.GenerateFromPassword(stream.StringToBytes(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		atomic.StoreUint32(&r.version, crc32.ChecksumIEEE(hashedPassword))
	}
	r.HashedPassword = hashedPassword
	return db.SetRoomHashedPassword(r.ID, hashedPassword)
}

func (r *Room) SetUserRole(userID string, role model.RoomRole) error {
	return db.SetUserRole(r.ID, userID, role)
}

func (r *Room) SetUserPermission(userID string, permission model.Permission) error {
	return db.SetUserPermission(r.ID, userID, permission)
}

func (r *Room) AddUserPermission(userID string, permission model.Permission) error {
	return db.AddUserPermission(r.ID, userID, permission)
}

func (r *Room) RemoveUserPermission(userID string, permission model.Permission) error {
	return db.RemoveUserPermission(r.ID, userID, permission)
}

func (r *Room) DeleteUserPermission(userID string) error {
	return db.DeleteUserPermission(r.ID, userID)
}

func (r *Room) GetMoviesCount() int {
	return r.movies.Len()
}

func (r *Room) DeleteMovieByID(id string) error {
	return r.movies.DeleteMovieByID(id)
}

func (r *Room) ClearMovies() error {
	return r.movies.Clear()
}

func (r *Room) GetMovieByID(id string) (*movie, error) {
	return r.movies.GetMovieByID(id)
}

func (r *Room) Current() *Current {
	c := r.current.Current()
	return &c
}

func (r *Room) ChangeCurrentMovie(id string) error {
	m, err := r.movies.GetMovieByID(id)
	if err != nil {
		return err
	}
	r.current.SetMovie(*m.Movie)
	return nil
}

func (r *Room) SwapMoviePositions(id1, id2 string) error {
	return r.movies.SwapMoviePositions(id1, id2)
}

func (r *Room) GetMoviesWithPage(page, pageSize int) []*movie {
	return r.movies.GetMoviesWithPage(page, pageSize)
}

func (r *Room) RegClient(user *User, conn *websocket.Conn) (*Client, error) {
	r.lazyInitHub()
	return r.hub.RegClient(newClient(user, r, conn))
}

func (r *Room) UnregisterClient(user *User) error {
	r.lazyInitHub()
	return r.hub.UnRegClient(user)
}

func (r *Room) SetStatus(playing bool, seek float64, rate float64, timeDiff float64) Status {
	return r.current.SetStatus(playing, seek, rate, timeDiff)
}

func (r *Room) SetSeekRate(seek float64, rate float64, timeDiff float64) Status {
	return r.current.SetSeekRate(seek, rate, timeDiff)
}

func (r *Room) SetRoomStatus(status model.RoomStatus) error {
	err := db.SetRoomStatus(r.ID, status)
	if err != nil {
		return err
	}
	r.Status = status
	switch status {
	case model.RoomStatusBanned, model.RoomStatusStopped, model.RoomStatusPending:
		return CompareAndCloseRoom(r)
	}
	return nil
}
