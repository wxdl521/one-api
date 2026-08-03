package controller

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProductControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Product{}, &model.ProductOrder{}, &model.SubscriptionPlan{}))
}

func TestAdminCreateProductRejectsSubscriptionWithoutPlan(t *testing.T) {
	setupProductControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/product/admin", bytes.NewBufferString(`{"name":"Agent Plan","product_type":"subscription"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	AdminCreateProduct(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestListProductsOnlyReturnsPublishedProducts(t *testing.T) {
	setupProductControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.Product{Name: "Published CDK", ProductType: model.ProductTypeManual, Enabled: true}).Error)
	require.NoError(t, model.DB.Create(&model.Product{Name: "Draft CDK", ProductType: model.ProductTypeManual, Enabled: false}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/product", nil)

	ListProducts(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Published CDK")
	assert.NotContains(t, recorder.Body.String(), "Draft CDK")
}

func TestCreateQRCodeProductOrderReturnsPendingOrder(t *testing.T) {
	setupProductControllerTestDB(t)
	product := model.Product{Name: "QR CDK", ProductType: model.ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, model.DB.Create(&product).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 7)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/product/orders", bytes.NewBufferString(`{"product_id":1,"payment_method":"qr_code"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CreateProductOrder(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"payment_status":"pending"`)
}

func TestAdminShipProductOrderRejectsUnpaidOrder(t *testing.T) {
	setupProductControllerTestDB(t)
	product := model.Product{Name: "QR CDK", ProductType: model.ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, model.DB.Create(&product).Error)
	order, err := model.CreateQRCodeProductOrder(7, product.Id)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/product/admin/orders/1/ship", nil)

	AdminShipProductOrder(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	assert.Equal(t, model.ProductOrderPaymentPending, order.PaymentStatus)
}

func TestAdminUploadProductImageStoresAllowedImage(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	require.NoError(t, err)
	workingDirectory := t.TempDir()
	require.NoError(t, os.Chdir(workingDirectory))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWorkingDirectory))
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "cover.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("png"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/product/admin/upload", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	AdminUploadProductImage(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"url":"/product-images/`)
	files, err := os.ReadDir(filepath.Join("data", "product-images"))
	require.NoError(t, err)
	assert.Len(t, files, 1)
}
