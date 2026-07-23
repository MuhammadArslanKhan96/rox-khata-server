package erp

import (
	"context"
	"fmt"
	"log"
)

type ERPService interface {
	SaveProduct(ctx context.Context, req CreateProductRequest) (*Product, error)
	GetDropdownData(ctx context.Context, businessID string) (*DropdownDataResponse, error)
	SaveCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error)
	SaveBrand(ctx context.Context, req CreateBrandRequest) (*Brand, error)
	SaveUnit(ctx context.Context, req CreateUnitRequest) (*Unit, error)
	SaveWarehouse(ctx context.Context, req CreateWarehouseRequest) (*Warehouse, error)
}

type erpService struct {
	repo ERPRepository
}

func NewERPService(repo ERPRepository) ERPService {
	return &erpService{repo: repo}
}

func (s *erpService) SaveProduct(ctx context.Context, req CreateProductRequest) (*Product, error) {
	// Begin atomic database transaction
	tx, err := s.repo.GetPool().Begin(ctx)
	if err != nil {
		log.Printf("[ERP Error] Failed to begin database transaction: %v", err)
		return nil, fmt.Errorf("transaction error: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx) // Safe no-op if committed
	}()

	// Save product and its opening stock in the transaction
	product, err := s.repo.SaveProduct(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("[ERP Error] Failed to commit database transaction: %v", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return product, nil
}

func (s *erpService) GetDropdownData(ctx context.Context, businessID string) (*DropdownDataResponse, error) {
	return s.repo.GetDropdownData(ctx, s.repo.GetPool(), businessID)
}

func (s *erpService) SaveCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	return s.repo.SaveCategory(ctx, s.repo.GetPool(), req)
}

func (s *erpService) SaveBrand(ctx context.Context, req CreateBrandRequest) (*Brand, error) {
	return s.repo.SaveBrand(ctx, s.repo.GetPool(), req)
}

func (s *erpService) SaveUnit(ctx context.Context, req CreateUnitRequest) (*Unit, error) {
	return s.repo.SaveUnit(ctx, s.repo.GetPool(), req)
}

func (s *erpService) SaveWarehouse(ctx context.Context, req CreateWarehouseRequest) (*Warehouse, error) {
	return s.repo.SaveWarehouse(ctx, s.repo.GetPool(), req)
}
