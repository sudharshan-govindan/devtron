/*
 * Copyright (c) 2020 Devtron Labs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package externalLink

import (
	"github.com/devtron-labs/devtron/internal/util"
	"github.com/devtron-labs/devtron/pkg/sql"
	"go.uber.org/zap"
	"time"
)

type ExternalLinkService interface {
	Create(requests []*ExternalLinkDto, userId int32) (*ExternalLinkApiResponse, error)
	GetAllActiveTools() ([]ExternalLinkMonitoringToolDto, error)
	FetchAllActiveLinks(clusterIds int) ([]*ExternalLinkDto, error)
	Update(request *ExternalLinkDto) (*ExternalLinkApiResponse, error)
	DeleteLink(id int, userId int32) (*ExternalLinkApiResponse, error)
}
type ExternalLinkServiceImpl struct {
	logger                               *zap.SugaredLogger
	externalLinkMonitoringToolRepository ExternalLinkMonitoringToolRepository
	externalLinkClusterMappingRepository ExternalLinkClusterMappingRepository
	externalLinkRepository               ExternalLinkRepository
}
type ExternalLinkMonitoringToolDto struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}
type ExternalLinkDto struct {
	Id               int       `json:"id"`
	Name             string    `json:"name"`
	Url              string    `json:"url"`
	Active           bool      `json:"active"`
	MonitoringToolId int       `json:"monitoringToolId"`
	ClusterIds       []int     `json:"clusterIds"`
	UpdatedOn        time.Time `json:"updatedOn"`
	UserId           int32     `json:"-"`
}

type ExternalLinkApiResponse struct {
	Success bool `json:"success"`
}

func NewExternalLinkServiceImpl(logger *zap.SugaredLogger, externalLinksToolsRepository ExternalLinkMonitoringToolRepository,
	externalLinksClustersRepository ExternalLinkClusterMappingRepository, externalLinksRepository ExternalLinkRepository) *ExternalLinkServiceImpl {
	return &ExternalLinkServiceImpl{
		logger:                               logger,
		externalLinkMonitoringToolRepository: externalLinksToolsRepository,
		externalLinkClusterMappingRepository: externalLinksClustersRepository,
		externalLinkRepository:               externalLinksRepository,
	}
}

func (impl ExternalLinkServiceImpl) Create(requests []*ExternalLinkDto, userId int32) (*ExternalLinkApiResponse, error) {
	impl.logger.Debugw("external links create request", "req", requests)
	for _, request := range requests {
		externalLink := &ExternalLink{
			Name:                         request.Name,
			Active:                       true,
			ExternalLinkMonitoringToolId: request.MonitoringToolId,
			Url:                          request.Url,
			AuditLog:                     sql.AuditLog{CreatedOn: time.Now(), CreatedBy: userId, UpdatedOn: time.Now(), UpdatedBy: userId},
		}
		err := impl.externalLinkRepository.Save(externalLink)
		if err != nil {
			impl.logger.Errorw("error in saving link", "data", externalLink, "err", err)
			err = &util.ApiError{
				InternalMessage: "external link failed to create in db",
				UserMessage:     "external link failed to create in db",
			}
			return nil, err
		}

		for _, clusterId := range request.ClusterIds {
			externalLinkClusterMapping := &ExternalLinkClusterMapping{
				ExternalLinkId: externalLink.Id,
				ClusterId:      clusterId,
				Active:         true,
				AuditLog:       sql.AuditLog{CreatedOn: time.Now(), CreatedBy: userId, UpdatedOn: time.Now(), UpdatedBy: userId},
			}
			err := impl.externalLinkClusterMappingRepository.Save(externalLinkClusterMapping)
			if err != nil {
				impl.logger.Errorw("error in saving cluster id's", "data", externalLink, "err", err)
				err = &util.ApiError{
					InternalMessage: "cluster id failed to create in db",
					UserMessage:     "cluster id failed to create in db",
				}
				return nil, err
			}
		}
	}
	externalLinksCreateUpdateResponse := &ExternalLinkApiResponse{
		Success: true,
	}
	return externalLinksCreateUpdateResponse, nil
}

func (impl ExternalLinkServiceImpl) GetAllActiveTools() ([]ExternalLinkMonitoringToolDto, error) {
	tools, err := impl.externalLinkMonitoringToolRepository.FindAllActive()
	if err != nil {
		impl.logger.Errorw("error in fetch all tools", "err", err)
		return nil, err
	}
	var response []ExternalLinkMonitoringToolDto
	for _, tool := range tools {
		providerRes := ExternalLinkMonitoringToolDto{
			Id:   tool.Id,
			Name: tool.Name,
			Icon: tool.Icon,
		}
		response = append(response, providerRes)
	}
	return response, err
}

func (impl ExternalLinkServiceImpl) FetchAllActiveLinks(clusterId int) ([]*ExternalLinkDto, error) {
	var err error
	var mappedExternalLinksIds []int
	filterByCluster := make(map[int]int)
	externalLinksMap := make(map[int]int)
	allActiveExternalLinkMapping, err := impl.externalLinkClusterMappingRepository.FindAllActive()
	for _, link := range allActiveExternalLinkMapping {
		externalLinksMap[link.ExternalLinkId] = link.ExternalLinkId
	}
	for _, externalLinksId := range externalLinksMap {
		mappedExternalLinksIds = append(mappedExternalLinksIds, externalLinksId)
	}

	var externalLinkResponse []*ExternalLinkDto
	response := make(map[int]*ExternalLinkDto)
	for _, link := range allActiveExternalLinkMapping {

		//requested all links
		if clusterId > 0 {
			if link.ClusterId == clusterId {
				filterByCluster[link.ExternalLinkId] = link.ExternalLinkId
			}
		}
		if _, ok := response[link.ExternalLinkId]; !ok {
			response[link.ExternalLinkId] = &ExternalLinkDto{
				Id:               link.ExternalLinkId,
				Name:             link.ExternalLinks.Name,
				Url:              link.ExternalLinks.Url,
				Active:           link.ExternalLinks.Active,
				MonitoringToolId: link.ExternalLinks.ExternalLinkMonitoringToolId,
				UpdatedOn:        link.UpdatedOn,
			}
		}
		response[link.ExternalLinkId].ClusterIds = append(response[link.ExternalLinkId].ClusterIds, link.ClusterId)
	}

	for k, v := range response {
		if _, ok := filterByCluster[k]; ok {
			externalLinkResponse = append(externalLinkResponse, v)
		} else if clusterId == 0 {
			externalLinkResponse = append(externalLinkResponse, v)
		}
	}

	//now add all the links which are not mapped to any clusters
	additionalExternalLinks, err := impl.externalLinkRepository.FindAllFilterOutByIds(mappedExternalLinksIds)
	if err != nil {
		impl.logger.Errorw("error in fetch all links", "err", err)
		return nil, err
	}
	for _, link := range additionalExternalLinks {
		providerRes := &ExternalLinkDto{
			Id:               link.Id,
			Name:             link.Name,
			Url:              link.Url,
			Active:           link.Active,
			MonitoringToolId: link.ExternalLinkMonitoringToolId,
			ClusterIds:       []int{},
			UpdatedOn:        link.UpdatedOn,
		}
		externalLinkResponse = append(externalLinkResponse, providerRes)
	}

	if externalLinkResponse == nil {
		externalLinkResponse = make([]*ExternalLinkDto, 0)
	}
	return externalLinkResponse, err
}
func (impl ExternalLinkServiceImpl) Update(request *ExternalLinkDto) (*ExternalLinkApiResponse, error) {
	impl.logger.Debugw("link update request", "req", request)
	externalLink, err0 := impl.externalLinkRepository.FindOne(request.Id)
	if err0 != nil {
		impl.logger.Errorw("No matching entry found for update.", "id", request.Id)
		return nil, err0
	}
	externalLink.Name = request.Name
	externalLink.Url = request.Url
	externalLink.Active = true
	externalLink.ExternalLinkMonitoringToolId = request.MonitoringToolId
	externalLink.UpdatedBy = int32(request.UserId)
	externalLink.UpdatedOn = time.Now()
	err := impl.externalLinkRepository.Update(&externalLink)
	if err != nil {
		impl.logger.Errorw("error in updating link", "data", externalLink, "err", err)
		return nil, err
	}

	allExternalLinksMapping, err := impl.externalLinkClusterMappingRepository.FindAllByExternalLinkId(request.Id)
	if err != nil {
		impl.logger.Errorw("error in fetching link", "data", externalLink, "err", err)
		return nil, err
	}
	for _, model := range allExternalLinksMapping {
		model.Active = false
		model.UpdatedBy = int32(request.UserId)
		model.UpdatedOn = time.Now()
		err := impl.externalLinkClusterMappingRepository.Update(model)
		if err != nil {
			impl.logger.Errorw("error in updating clusters to false", "data", model, "err", err)
			return nil, err
		}
	}
	for _, requestedClusterId := range request.ClusterIds {
		externalLinkClusterId := 0
		var externalLinkCluster *ExternalLinkClusterMapping
		for _, model := range allExternalLinksMapping {
			if requestedClusterId == model.ClusterId {
				externalLinkClusterId = model.Id
				externalLinkCluster = model
				break
			}
		}
		if externalLinkClusterId > 0 && externalLinkCluster != nil {
			externalLinkCluster.Active = true
			externalLinkCluster.UpdatedOn = time.Now()
			externalLinkCluster.UpdatedBy = request.UserId
			err = impl.externalLinkClusterMappingRepository.Update(externalLinkCluster)
		} else {
			externalLinkCluster := &ExternalLinkClusterMapping{
				ExternalLinkId: request.Id,
				ClusterId:      requestedClusterId,
				Active:         true,
				AuditLog:       sql.AuditLog{CreatedOn: time.Now(), CreatedBy: request.UserId, UpdatedOn: time.Now(), UpdatedBy: request.UserId},
			}
			err = impl.externalLinkClusterMappingRepository.Save(externalLinkCluster)
		}
		if err != nil {
			impl.logger.Errorw("error in saving cluster id's", "data", externalLinkCluster, "err", err)
			err = &util.ApiError{
				InternalMessage: "cluster id failed to create in db",
				UserMessage:     "cluster id failed to create in db",
			}
			return nil, err
		}
	}
	externalLinksCreateUpdateResponse := &ExternalLinkApiResponse{
		Success: true,
	}
	return externalLinksCreateUpdateResponse, nil
}
func (impl ExternalLinkServiceImpl) DeleteLink(id int, userId int32) (*ExternalLinkApiResponse, error) {
	impl.logger.Debugw("link delete request", "req", id)
	externalLinksClusterMapping, err := impl.externalLinkClusterMappingRepository.FindAllActiveByExternalLinkId(id)
	if err != nil {
		return nil, err
	}
	for _, externalLink := range externalLinksClusterMapping {
		externalLink.Active = false
		externalLink.UpdatedOn = time.Now()
		externalLink.UpdatedBy = userId
		err := impl.externalLinkClusterMappingRepository.Update(externalLink)
		if err != nil {
			impl.logger.Errorw("error in deleting clusters to false", "data", externalLink, "err", err)
			return nil, err
		}
	}

	externalLink, err := impl.externalLinkRepository.FindOne(id)
	if err != nil {
		return nil, err
	}
	externalLink.Active = false
	externalLink.UpdatedOn = time.Now()
	externalLink.UpdatedBy = userId
	err = impl.externalLinkRepository.Update(&externalLink)
	if err != nil {
		impl.logger.Errorw("error in deleting link", "data", externalLink, "err", err)
		return nil, err
	}

	externalLinksCreateUpdateResponse := &ExternalLinkApiResponse{
		Success: true,
	}
	return externalLinksCreateUpdateResponse, nil
}
