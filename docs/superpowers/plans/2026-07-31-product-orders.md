# Product Orders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add RMB-priced product orders, wallet and QR-code payment, manual fulfilment, and administrator image uploads.

**Architecture:** Extend `Product` with purchase settings and add a `ProductOrder` snapshot. Model transactions own wallet debits, payment confirmation, refunds, and fulfilment state transitions; controllers bind validated requests and enforce permissions. Administrators upload images before saving product URLs.

**Tech Stack:** Go, Gin, GORM, SQLite/MySQL/PostgreSQL, React 19, TypeScript, React Query, React Hook Form, Zod, Bun.

---

### Task 1: Define product and order model contracts

**Files:**
- Modify: `model/product.go`
- Create: `model/product_order.go`
- Modify: `model/main.go`
- Create: `model/product_order_test.go`

- [ ] **Step 1: Write the failing model tests**

```go
func TestProductNormalizeRejectsNonPositivePrice(t *testing.T) {
	product := Product{Name: "Manual CDK", ProductType: ProductTypeManual, PriceCents: 0}
	require.EqualError(t, product.Normalize(), "product price must be greater than zero")
}

func TestCreateQRCodeProductOrderSnapshotsProduct(t *testing.T) {
	product := Product{Name: "Agent plan", ProductType: ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, DB.Create(&product).Error)
	order, err := CreateQRCodeProductOrder(42, product.Id)
	require.NoError(t, err)
	assert.Equal(t, ProductOrderPaymentPending, order.PaymentStatus)
	assert.Equal(t, "Agent plan", order.ProductName)
	assert.Equal(t, 9900, order.PriceCents)
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run: `go test ./model -run 'TestProductNormalizeRejectsNonPositivePrice|TestCreateQRCodeProductOrderSnapshotsProduct' -count=1`

Expected: FAIL because `PriceCents`, `ProductOrder`, and `CreateQRCodeProductOrder` do not exist.

- [ ] **Step 3: Implement the schema and QR order creation**

```go
// Product fields: PriceCents int, PaymentQRCodeURL string, PaymentInstructions string.
// ProductOrder fields: UserId, ProductId, ProductName, PriceCents, PaymentMethod,
// PaymentStatus, FulfillmentStatus, PaidQuota, CreatedAt and UpdatedAt.
const (
	ProductOrderPaymentWallet = "wallet"
	ProductOrderPaymentQRCode = "qr_code"
	ProductOrderPaymentPending = "pending"
	ProductOrderPaymentPaid = "paid"
	ProductOrderFulfillmentPending = "pending"
	ProductOrderFulfillmentShipped = "shipped"
	ProductOrderFulfillmentCancelled = "cancelled"
)
```

Require `PriceCents > 0` in `Product.Normalize`, validate QR URL and instruction lengths, add `&ProductOrder{}` to normal and fast GORM migration lists, then implement `CreateQRCodeProductOrder` inside a transaction using `lockForUpdate(tx)` and an enabled product query.

- [ ] **Step 4: Run the model tests to verify GREEN**

Run: `go test ./model -run 'TestProductNormalizeRejectsNonPositivePrice|TestCreateQRCodeProductOrderSnapshotsProduct' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the model contract**

```bash
git add model/product.go model/product_order.go model/main.go model/product_order_test.go
git commit -m "feat: add product order models"
```

### Task 2: Implement safe wallet payment and manual order transitions

**Files:**
- Modify: `model/product_order.go`
- Modify: `model/product_order_test.go`

- [ ] **Step 1: Write failing lifecycle tests**

```go
func TestCreateWalletProductOrderDebitsBalance(t *testing.T) {
	user := User{Username: "buyer", Status: common.UserStatusEnabled, Quota: walletQuotaForCNYCents(9900)}
	require.NoError(t, DB.Create(&user).Error)
	product := Product{Name: "CDK", ProductType: ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, DB.Create(&product).Error)
	order, err := CreateWalletProductOrder(user.Id, product.Id)
	require.NoError(t, err)
	assert.Equal(t, ProductOrderPaymentPaid, order.PaymentStatus)
}

func TestCreateWalletProductOrderDoesNotCreateOrderWhenBalanceIsInsufficient(t *testing.T) {
	_, err := CreateWalletProductOrder(42, 9)
	require.ErrorIs(t, err, ErrProductOrderInsufficientBalance)
	var count int64
	DB.Model(&ProductOrder{}).Count(&count)
	assert.Zero(t, count)
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run: `go test ./model -run 'TestCreateWalletProductOrder|TestConfirmProductOrder|TestCancelPaidProductOrder' -count=1`

Expected: FAIL because wallet payment and state transition functions are missing.

- [ ] **Step 3: Implement transaction-backed lifecycle functions**

```go
func CreateWalletProductOrder(userId, productId int) (*ProductOrder, error)
func ConfirmQRCodeProductOrder(orderId int) error
func ShipProductOrder(orderId int) error
func CancelProductOrder(orderId int) error
```

Lock product, user, and order rows with `lockForUpdate(tx)`. Convert CNY cents to quota with decimal arithmetic and `common.QuotaFromDecimalChecked`; reject wallet payment when CNY conversion is unavailable or the balance is insufficient. In one transaction insert the paid order and debit quota. Refund `PaidQuota` exactly once only for paid wallet orders when cancelling.

- [ ] **Step 4: Run lifecycle tests to verify GREEN**

Run: `go test ./model -run 'TestCreateWalletProductOrder|TestConfirmProductOrder|TestCancelPaidProductOrder' -count=1`

Expected: PASS; the cancellation test proves a second cancel does not credit the wallet twice.

- [ ] **Step 5: Commit wallet safety logic**

```bash
git add model/product_order.go model/product_order_test.go
git commit -m "feat: add product wallet payments"
```

### Task 3: Add image-upload and order APIs

**Files:**
- Create: `controller/product_order.go`
- Create: `controller/product_upload.go`
- Modify: `controller/product.go`
- Modify: `controller/product_test.go`
- Modify: `router/api-router.go`
- Modify: `router/web-router.go`

- [ ] **Step 1: Write failing controller tests**

```go
func TestCreateQRCodeProductOrderReturnsPendingOrder(t *testing.T) {
	ctx, recorder := newAuthenticatedProductContext(http.MethodPost, "/api/product/orders", `{"product_id":1,"payment_method":"qr_code"}`, 7)
	CreateProductOrder(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"payment_status":"pending"`)
}

func TestAdminShipProductOrderRejectsUnpaidOrder(t *testing.T) {
	ctx, recorder := newAuthenticatedProductContext(http.MethodPatch, "/api/product/admin/orders/1/ship", "", 1)
	AdminShipProductOrder(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
}
```

- [ ] **Step 2: Run controller tests to verify RED**

Run: `go test ./controller -run 'TestCreateQRCodeProductOrderReturnsPendingOrder|TestAdminShipProductOrderRejectsUnpaidOrder' -count=1`

Expected: FAIL because handlers are missing.

- [ ] **Step 3: Implement upload and route handlers**

```go
func AdminUploadProductImage(c *gin.Context) {
	file, err := c.FormFile("file")
	// Accept JPEG, PNG, WebP and GIF only, reject oversized files, generate
	// an unpredictable filename below data/product-images, and return its URL.
}
```

Serve only `data/product-images` at `/product-images`. Apply `AdminAuth` to image and admin order endpoints. Apply `UserAuth` to checkout and current-user orders. Add create/list user order, list admin order, confirm, ship, and cancel handlers. QR checkout must return the stored QR URL and payment instructions; no payment-proof upload is added.

- [ ] **Step 4: Run controller tests to verify GREEN**

Run: `go test ./controller -run 'TestCreateQRCodeProductOrderReturnsPendingOrder|TestAdminShipProductOrderRejectsUnpaidOrder' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit API and upload endpoints**

```bash
git add controller/product.go controller/product_order.go controller/product_upload.go controller/product_test.go router/api-router.go router/web-router.go
git commit -m "feat: add product order APIs"
```

### Task 4: Build management, checkout, and customer order UI

**Files:**
- Modify: `web/src/features/products/types.ts`
- Modify: `web/src/features/products/api.ts`
- Modify: `web/src/features/products/lib/product-form.ts`
- Modify: `web/src/features/products/__tests__/validation.test.ts`
- Modify: `web/src/features/products/product-management.tsx`
- Modify: `web/src/features/products/product-detail.tsx`
- Create: `web/src/features/products/orders.tsx`
- Create: `web/src/routes/_authenticated/products/orders.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts`

- [ ] **Step 1: Write failing frontend validation tests**

```ts
test('requires a positive whole-number RMB-cent product price', () => {
  expect(productFormSchema.safeParse({ ...validProduct, price_cents: 0 }).success).toBe(false)
})

test('accepts QR-code payment for a product order', () => {
  expect(productOrderSchema.safeParse({ product_id: 1, payment_method: 'qr_code' }).success).toBe(true)
})
```

- [ ] **Step 2: Run the test to verify RED**

Run: `bun test src/features/products/__tests__/validation.test.ts`

Expected: FAIL because the price and order schemas are absent.

- [ ] **Step 3: Implement typed APIs, uploads, and pages**

```ts
export async function uploadProductImage(file: File) {
  const data = new FormData()
  data.append('file', file)
  return (await api.post('/api/product/admin/upload', data)).data
}

export async function createProductOrder(data: ProductOrderPayload) {
  return (await api.post('/api/product/orders', data)).data
}
```

Add integer-cent price, cover upload, QR upload, and payment-instruction fields to product management. Format price as `¥xx.xx` in lists and details. On product detail, offer wallet and QR payment; after QR checkout show the response QR code and instructions. Add “My orders” to the personal sidebar and list only the current user's orders and statuses.

- [ ] **Step 4: Run frontend tests to verify GREEN**

Run: `bun test src/features/products/__tests__/validation.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit customer product flow**

```bash
git add web/src/features/products web/src/routes/_authenticated/products web/src/hooks/use-sidebar-data.ts web/src/routeTree.gen.ts
git commit -m "feat: add product checkout flow"
```

### Task 5: Build administrator order management and translations

**Files:**
- Create: `web/src/features/products/product-orders-management.tsx`
- Create: `web/src/routes/_authenticated/products/orders.manage.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts`
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/scripts/add-missing-keys.mjs`
- Modify: locale JSON through the script only

- [ ] **Step 1: Write the failing status-label test**

```ts
test('maps a paid pending order to the manual fulfilment label', () => {
  expect(getOrderStatusLabel({ payment_status: 'paid', fulfillment_status: 'pending' })).toBe('Awaiting manual delivery')
})
```

- [ ] **Step 2: Run the test to verify RED**

Run: `bun test src/features/products/__tests__/validation.test.ts`

Expected: FAIL because `getOrderStatusLabel` does not exist.

- [ ] **Step 3: Implement the admin order page and translations**

```tsx
<Button onClick={() => confirmOrder.mutate(order.id)} disabled={order.payment_status !== 'pending'}>
  {t('Confirm payment')}
</Button>
<Button onClick={() => shipOrder.mutate(order.id)} disabled={order.payment_status !== 'paid' || order.fulfillment_status !== 'pending'}>
  {t('Mark as shipped')}
</Button>
```

Add an Admin “Product Orders” route and sidebar entry. Add every new UI key to `static-keys.ts`, populate all seven locales through `web/scripts/add-missing-keys.mjs`, and run `bun run i18n:sync`. Do not manually edit locale JSON files.

- [ ] **Step 4: Run the frontend test to verify GREEN**

Run: `bun test src/features/products/__tests__/validation.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit administrator order UI**

```bash
git add web/src/features/products web/src/routes/_authenticated/products web/src/hooks/use-sidebar-data.ts web/src/i18n/static-keys.ts web/src/i18n/locales web/src/routeTree.gen.ts
git commit -m "feat: add product order management"
```

### Task 6: Verify migrations, lifecycle, and production build

**Files:**
- Modify: only files required by verification failures

- [ ] **Step 1: Run focused Go tests**

Run: `go test ./model ./controller -run 'Product' -count=1`

Expected: PASS.

- [ ] **Step 2: Run all backend tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run frontend checks**

Run: `bun test src/features/products/__tests__/validation.test.ts && bun run typecheck && bun run build`

Expected: PASS.

- [ ] **Step 4: Validate a fresh local database manually**

Run: `go run .`

Expected: migrations create `products` and `product_orders`; an administrator uploads a PNG, creates a ¥99.00 product, users can create wallet and QR orders, and an administrator can confirm, ship, or cancel them.

## Self-review

- Every accepted requirement has a task: RMB price, wallet payment, QR payment, manual delivery, product/QR uploads, user orders, and administrator order management.
- Model, controller, and frontend names are consistent across the plan.
- Automatic CDK issuance, subscription activation, payment-proof uploads, and external payment gateways are explicitly out of scope.
