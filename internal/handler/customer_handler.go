package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/response"
	"github.com/payfake/payfake-api/internal/service"
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

// Create handles POST /customer
func (h *CustomerHandler) Create(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
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
			response.Error(c, http.StatusConflict,
				"Customer with that email already exists",
				response.CustomerEmailTaken,
				field("email", "unique", "A customer with this email already exists"))
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Customer created",
		response.CustomerCreated, buildCustomerData(customer))
}

// List handles GET /customer
func (h *CustomerHandler) List(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("perPage", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	customers, total, err := h.customerSvc.List(merchant.ID, page, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	var data []gin.H
	for i := range customers {
		data = append(data, buildCustomerData(&customers[i]))
	}
	if data == nil {
		data = []gin.H{}
	}

	response.SuccessList(c, "Customers retrieved",
		response.CustomerListFetched,
		data,
		response.BuildPaystackMeta(total, page, perPage))
}

// Fetch handles GET /customer/:code
func (h *CustomerHandler) Fetch(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	customer, err := h.customerSvc.Get(c.Param("code"), merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrCustomerNotFound) {
			response.NotFoundErr(c, "Customer not found")
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Customer retrieved",
		response.CustomerFetched, buildCustomerData(customer))
}

type updateCustomerRequest struct {
	FirstName *string     `json:"first_name"`
	LastName  *string     `json:"last_name"`
	Phone     *string     `json:"phone"`
	Metadata  domain.JSON `json:"metadata"`
}

// Update handles PUT /customer/:code
func (h *CustomerHandler) Update(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	var req updateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	customer, err := h.customerSvc.Update(c.Param("code"), merchant.ID, service.UpdateCustomerInput{
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
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Customer updated",
		response.CustomerUpdated, buildCustomerData(customer))
}

// Transactions handles GET /customer/:code/transactions
func (h *CustomerHandler) Transactions(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	customer, err := h.customerSvc.Get(c.Param("code"), merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrCustomerNotFound) {
			response.NotFoundErr(c, "Customer not found")
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("perPage", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	transactions, total, err := h.txSvc.ListByCustomer(customer.ID, merchant.ID, page, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	var data []gin.H
	for i := range transactions {
		data = append(data, buildTransactionData(&transactions[i], nil))
	}
	if data == nil {
		data = []gin.H{}
	}

	response.SuccessList(c, "Customer transactions retrieved",
		response.TransactionListFetched,
		data,
		response.BuildPaystackMeta(total, page, perPage))
}
