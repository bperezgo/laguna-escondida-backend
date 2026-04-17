package dto

import "time"

type BillCriteria struct {
	Page                   int
	PageSize               int
	CreatedAtStart         *time.Time
	CreatedAtEnd           *time.Time
	NationalIdentification *string
}

func NewBillCriteria() *BillCriteria {
	return &BillCriteria{
		Page:     1,
		PageSize: 20,
	}
}

func (c *BillCriteria) WithPage(page int) *BillCriteria {
	if page > 0 {
		c.Page = page
	}
	return c
}

func (c *BillCriteria) WithPageSize(pageSize int) *BillCriteria {
	if pageSize > 0 && pageSize <= 100 {
		c.PageSize = pageSize
	}
	return c
}

func (c *BillCriteria) WithCreatedAtRange(start, end *time.Time) *BillCriteria {
	c.CreatedAtStart = start
	c.CreatedAtEnd = end
	return c
}

func (c *BillCriteria) WithNationalIdentification(nationalIdentification *string) *BillCriteria {
	c.NationalIdentification = nationalIdentification
	return c
}

func (c *BillCriteria) GetOffset() int {
	return (c.Page - 1) * c.PageSize
}

func (c *BillCriteria) GetLimit() int {
	return c.PageSize
}

type SupportDocumentCriteria struct {
	Page                   int
	PageSize               int
	CreatedAtStart         *time.Time
	CreatedAtEnd           *time.Time
	ProviderDocumentNumber *string
}

func NewSupportDocumentCriteria() *SupportDocumentCriteria {
	return &SupportDocumentCriteria{
		Page:     1,
		PageSize: 20,
	}
}

func (c *SupportDocumentCriteria) WithPage(page int) *SupportDocumentCriteria {
	if page > 0 {
		c.Page = page
	}
	return c
}

func (c *SupportDocumentCriteria) WithPageSize(pageSize int) *SupportDocumentCriteria {
	if pageSize > 0 && pageSize <= 100 {
		c.PageSize = pageSize
	}
	return c
}

func (c *SupportDocumentCriteria) WithCreatedAtRange(start, end *time.Time) *SupportDocumentCriteria {
	c.CreatedAtStart = start
	c.CreatedAtEnd = end
	return c
}

func (c *SupportDocumentCriteria) WithProviderDocumentNumber(providerDocumentNumber *string) *SupportDocumentCriteria {
	c.ProviderDocumentNumber = providerDocumentNumber
	return c
}

func (c *SupportDocumentCriteria) GetOffset() int {
	return (c.Page - 1) * c.PageSize
}

func (c *SupportDocumentCriteria) GetLimit() int {
	return c.PageSize
}
