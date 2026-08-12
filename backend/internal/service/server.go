package service

import (
	"errors"

	"cloudmanager/internal/auth"
	"cloudmanager/internal/cloud"
	"cloudmanager/internal/model"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

type ServerService struct {
	db  *gorm.DB
	enc *auth.AESEncryptor
}

func NewServerService(db *gorm.DB, enc *auth.AESEncryptor) *ServerService {
	return &ServerService{db: db, enc: enc}
}

func (s *ServerService) Paginate(q model.ServerQuery) (*model.PageResult, error) {
	q.Normalize()
	return repository.NewServerRepository(s.db).Paginate(q)
}

func (s *ServerService) FindByID(id uint) (*model.Server, error) {
	return repository.NewServerRepository(s.db).FindByID(id)
}

// ListTags 全部标签（去重）
func (s *ServerService) ListTags() ([]string, error) {
	return repository.NewServerRepository(s.db).ListTags()
}

// UpdateMeta 更新用户可编辑字段：自定义名称、标签
func (s *ServerService) UpdateMeta(id uint, customName, tags string) error {
	return repository.NewServerRepository(s.db).UpdateMeta(id, customName, tags)
}

// BatchAction 按筛选条件（含标签）对全部命中服务器执行操作，返回影响数量
func (s *ServerService) BatchAction(req model.ServerBatchActionRequest) (int, error) {
	ids, err := repository.NewServerRepository(s.db).ListIDs(model.ServerQuery{
		PageQuery: model.PageQuery{Keyword: req.Keyword},
		Provider:  req.Provider,
		Region:    req.Region,
		Status:    req.Status,
		Tag:       req.Tag,
	})
	if err != nil {
		return 0, err
	}
	var lastErr error
	for _, id := range ids {
		if err := s.Action(id, req.Action); err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return len(ids), lastErr
	}
	return len(ids), nil
}


// ListRegions 返回所有出现过的地域（筛选项动态填充用）
func (s *ServerService) ListRegions() ([]string, error) {
	return repository.NewServerRepository(s.db).ListRegions()
}

// Action 执行服务器操作：start / stop / reboot
// 调用云厂商接口，并更新本地缓存状态
func (s *ServerService) Action(id uint, action string) error {
	srvRepo := repository.NewServerRepository(s.db)
	srv, err := srvRepo.FindByID(id)
	if err != nil {
		return errors.New("服务器不存在")
	}

	// 解析云账号
	acctRepo := repository.NewCloudAccountRepository(s.db)
	acct, err := acctRepo.FindByID(srv.CloudAccountID)
	if err != nil {
		return errors.New("云账号不存在")
	}
	secret, err := s.enc.Decrypt(acct.AccessKeySecret)
	if err != nil {
		secret = ""
	}
	account := cloud.Account{
		ID:              acct.ID,
		Provider:        acct.Provider,
		AccessKeyID:     acct.AccessKeyID,
		AccessKeySecret: secret,
		Region:          acct.Region,
	}
	provider := cloud.Get(acct.Provider)
	if provider == nil {
		return errors.New("不支持的云厂商: " + acct.Provider)
	}

	newStatus := ""
	switch action {
	case "start":
		err = provider.StartServer(nil, account, srv.InstanceID, srv.Region, srv.Category)
		newStatus = "Running"
	case "stop":
		err = provider.StopServer(nil, account, srv.InstanceID, srv.Region, srv.Category)
		newStatus = "Stopped"
	case "reboot":
		err = provider.RebootServer(nil, account, srv.InstanceID, srv.Region, srv.Category)
		newStatus = "Running"
	default:
		return errors.New("不支持的操作")
	}
	if err != nil {
		return err
	}
	// 更新本地缓存状态，前端立即可见
	return srvRepo.UpdateStatus(id, newStatus)
}
