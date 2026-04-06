package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/GordenArcher/payfake/internal/response"
	"github.com/GordenArcher/payfake/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CustomerHandler struct {
	db          *gorm.DB
	customerSvc *service.CustomerService
	txSvc       *service.TransactionService
}

func NewCustomerHandler(db *gorm.DB, customerSvc *service.CustomerService, txSvc *service.TransactionService) *CustomerHandler {
	return &CustomerHandler{db: db, customerSvc: customerSvc, txSvc: txSvc}
}

type createCustomerRequest struct {
	Email     string      `json:"email" binding:"required,email"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Phone     string      `json:"phone"`
	Metadata  domain.JSON `json:"metadata"`
}

// Create handles POST /api/v1/customer
func (h *CustomerHandler) Create(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req createCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	customer, err := h.customerSvc.Create(service.CreateCustomerInput{
		MerchantID: merchant.ID,
		Email:      req.Email,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Phone:      req.Phone,
		Metadata:   req.Metadata,
	})

	if err != nil {
		if errors.Is(err, service.ErrCustomerEmailTaken) {
			response.Error(c, http.StatusConflict, "Customer email already exists",
				response.CustomerEmailTaken, []response.ErrorField{
					{Field: "email", Message: "A customer with this email already exists"},
				})
			return
		}
		response.InternalErr(c, "Failed to create customer")
		return
	}

	response.Success(c, http.StatusCreated, "Customer created",
		response.CustomerCreated, customer)
}

// List handles GET /api/v1/customer
func (h *CustomerHandler) List(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	customers, total, err := h.customerSvc.List(merchant.ID, page, perPage)
	if err != nil {
		response.InternalErr(c, "Failed to fetch customers")
		return
	}

	response.Success(c, http.StatusOK, "Customers fetched",
		response.CustomerListFetched, gin.H{
			"customers": customers,
			"meta": gin.H{
				"total":    total,
				"page":     page,
				"per_page": perPage,
				"pages":    (total + int64(perPage) - 1) / int64(perPage),
			},
		})
}

// Fetch handles GET /api/v1/customer/:code
func (h *CustomerHandler) Fetch(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	code := c.Param("code")

	customer, err := h.customerSvc.Get(code, merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrCustomerNotFound) {
			response.NotFoundErr(c, "Customer not found")
			return
		}
		response.InternalErr(c, "Failed to fetch customer")
		return
	}

	response.Success(c, http.StatusOK, "Customer fetched",
		response.CustomerFetched, customer)
}

type updateCustomerRequest struct {
	FirstName *string     `json:"first_name"`
	LastName  *string     `json:"last_name"`
	Phone     *string     `json:"phone"`
	Metadata  domain.JSON `json:"metadata"`
}

// Update handles PUT /api/v1/customer/:code
func (h *CustomerHandler) Update(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	code := c.Param("code")

	var req updateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	customer, err := h.customerSvc.Update(code, merchant.ID, service.UpdateCustomerInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Metadata:  req.Metadata,
	})

	if err != nil {
		if errors.Is(err, service.ErrCustomerNotFound) {
			response.NotFoundErr(c, "Customer not found")
			return
		}
		response.InternalErr(c, "Failed to update customer")
		return
	}

	response.Success(c, http.StatusOK, "Customer updated",
		response.CustomerUpdated, customer)
}

// Transactions handles GET /api/v1/customer/:code/transactions
func (h *CustomerHandler) Transactions(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	code := c.Param("code")

	// Verify the customer exists under this merchant before fetching their transactions.
	customer, err := h.customerSvc.Get(code, merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrCustomerNotFound) {
			response.NotFoundErr(c, "Customer not found")
			return
		}
		response.InternalErr(c, "Failed to fetch customer")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	transactions, total, err := h.txSvc.ListByCustomer(customer.ID, merchant.ID, page, perPage)
	if err != nil {
		response.InternalErr(c, "Failed to fetch transactions")
		return
	}

	response.Success(c, http.StatusOK, "Customer transactions fetched",
		response.TransactionListFetched, gin.H{
			"transactions": transactions,
			"meta": gin.H{
				"total":    total,
				"page":     page,
				"per_page": perPage,
				"pages":    (total + int64(perPage) - 1) / int64(perPage),
			},
		})
}
