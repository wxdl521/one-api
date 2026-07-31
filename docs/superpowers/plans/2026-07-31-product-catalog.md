# Product Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an independent product catalog, detail page, and administrator publishing workflow without checkout or delivery automation.

**Architecture:** A new GORM Product model holds public catalog content and optionally references an existing SubscriptionPlan. Gin exposes separate user and admin endpoints; a React products feature consumes those endpoints.

**Tech Stack:** Go, Gin, GORM, React, TypeScript, TanStack Router/Query, React Hook Form, Zod, i18next.

---

### Task 1: Product model and migration

**Files:**
- Create: `model/product.go`
- Create: `model/product_test.go`
- Modify: `model/main.go`

- [ ] **Step 1: Write the failing test**

```go
func TestListPublishedProductsExcludesUnpublished(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Product{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Product{}).Error)
	require.NoError(t, DB.Create(&Product{Name: "Visible", ProductType: ProductTypeManual, Enabled: true}).Error)
	require.NoError(t, DB.Create(&Product{Name: "Hidden", ProductType: ProductTypeManual, Enabled: false}).Error)
	products, err := ListPublishedProducts()
	require.NoError(t, err)
	require.Len(t, products, 1)
	assert.Equal(t, "Visible", products[0].Name)
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./model -run TestListPublishedProductsExcludesUnpublished -count=1`

Expected: FAIL because Product has not been implemented.

- [ ] **Step 3: Implement the minimal model**

```go
const (
	ProductTypeManual = "manual"
	ProductTypeSubscription = "subscription"
)

type Product struct {
	Id int `json:"id"`
	Name string `json:"name" gorm:"type:varchar(128);not null"`
	Summary string `json:"summary" gorm:"type:varchar(255);default:''"`
	Description string `json:"description" gorm:"type:text"`
	ImageURL string `json:"image_url" gorm:"type:varchar(2048);default:''"`
	ProductType string `json:"product_type" gorm:"type:varchar(32);not null"`
	SubscriptionPlanId *int `json:"subscription_plan_id,omitempty" gorm:"index"`
	Enabled bool `json:"enabled"`
	SortOrder int `json:"sort_order" gorm:"type:int;default:0"`
	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func ListPublishedProducts() ([]Product, error) {
	products := make([]Product, 0)
	err := DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&products).Error
	return products, err
}
```

Add `&Product{}` in the normal and fast migration paths, with timestamp hooks matching `SubscriptionPlan`.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./model -run TestListPublishedProductsExcludesUnpublished -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model/product.go model/product_test.go model/main.go
git commit -m "feat: add product catalog model"
```

### Task 2: Product API

**Files:**
- Create: `controller/product.go`
- Create: `controller/product_test.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write the failing API tests**

```go
func TestCreateProductRejectsSubscriptionWithoutPlan(t *testing.T) {
	response := performJSONRequest(setupTestRouterWithAdmin(), http.MethodPost, "/api/product/admin", `{"name":"Agent Plan","product_type":"subscription"}`)
	require.Equal(t, http.StatusOK, response.Code)
	assert.False(t, responseBodySuccess(t, response))
}

func TestGetProductsOnlyReturnsPublishedRecords(t *testing.T) {
	require.NoError(t, model.DB.Create(&model.Product{Name: "Published CDK", ProductType: model.ProductTypeManual, Enabled: true}).Error)
	require.NoError(t, model.DB.Create(&model.Product{Name: "Draft CDK", ProductType: model.ProductTypeManual, Enabled: false}).Error)
	response := performJSONRequest(setupTestRouterWithUser(), http.MethodGet, "/api/product", "")
	assert.Contains(t, response.Body.String(), "Published CDK")
	assert.NotContains(t, response.Body.String(), "Draft CDK")
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./controller -run 'Test(CreateProductRejectsSubscriptionWithoutPlan|GetProductsOnlyReturnsPublishedRecords)' -count=1`

Expected: FAIL because product routes do not exist.

- [ ] **Step 3: Implement the handlers and routes**

Add authenticated user `GET /api/product`, `GET /api/product/:id`; add administrator `GET/POST/PUT /api/product/admin` and `PATCH /api/product/admin/:id/status`. Validate type, trim all text, require a nonempty name, accept one optional plan only for subscription products, and remove the plan reference from manual products. The user endpoints always filter `enabled = true`.

```go
productRoute := apiRouter.Group("/product")
productRoute.Use(middleware.UserAuth())
productRoute.GET("", controller.ListProducts)
productRoute.GET("/:id", controller.GetProduct)
productAdminRoute := productRoute.Group("/admin")
productAdminRoute.Use(middleware.AdminAuth())
productAdminRoute.GET("", controller.AdminListProducts)
productAdminRoute.POST("", controller.AdminCreateProduct)
productAdminRoute.PUT("/:id", controller.AdminUpdateProduct)
productAdminRoute.PATCH("/:id/status", controller.AdminUpdateProductStatus)
```

- [ ] **Step 4: Verify GREEN**

Run: `go test ./controller -run 'Test(CreateProductRejectsSubscriptionWithoutPlan|GetProductsOnlyReturnsPublishedRecords)' -count=1`

Expected: PASS.

### Task 3: Product feature client and management form

**Files:**
- Create: `web/src/features/products/types.ts`
- Create: `web/src/features/products/api.ts`
- Create: `web/src/features/products/lib/product-form.ts`
- Create: `web/src/features/products/__tests__/validation.test.ts`

- [ ] **Step 1: Write the failing form test**

```ts
test('subscription product requires a subscription plan', () => {
  const result = productFormSchema.safeParse({
    name: 'Volcengine Agent Plan', product_type: 'subscription', enabled: true,
    summary: '', description: '', image_url: '', sort_order: 0,
  })
  expect(result.success).toBe(false)
})
```

- [ ] **Step 2: Verify RED**

Run: `cd web && bun x vitest run src/features/products/__tests__/validation.test.ts`

Expected: FAIL because the form schema does not exist.

- [ ] **Step 3: Implement types, client and schema**

Define `ProductType = 'manual' | 'subscription'`, Product API types, list/detail/admin API functions, and a Zod schema that requires `subscription_plan_id` only for subscription type. The manual path clears that field before submit.

- [ ] **Step 4: Verify GREEN**

Run: `cd web && bun x vitest run src/features/products/__tests__/validation.test.ts`

Expected: PASS.

### Task 4: Catalog, detail, and management UI

**Files:**
- Create: `web/src/features/products/index.tsx`
- Create: `web/src/features/products/product-detail.tsx`
- Create: `web/src/features/products/product-management.tsx`
- Create: `web/src/features/products/components/product-form-drawer.tsx`
- Create: `web/src/routes/_authenticated/products/index.tsx`
- Create: `web/src/routes/_authenticated/products/$productId.tsx`
- Create: `web/src/routes/_authenticated/products/manage.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts`
- Modify: `web/src/routeTree.gen.ts`

- [ ] **Step 1: Write the failing catalog test**

```tsx
test('catalog links to a product detail view', async () => {
  render(<Products />, { wrapper: createQueryWrapper() })
  expect(await screen.findByRole('link', { name: 'View details' })).toBeVisible()
})
```

- [ ] **Step 2: Verify RED**

Run: `cd web && bun x vitest run src/features/products/__tests__/catalog.test.tsx`

Expected: FAIL because Products does not exist.

- [ ] **Step 3: Implement focused components**

Build a responsive card catalog and dedicated detail page. Manual goods render the approved `Contact administrator to purchase` guidance; subscription goods link to the existing subscription management page. The admin page offers create/edit in a form drawer, shows all status values, and uses the status endpoint to publish/unpublish. Add Products to the personal sidebar and Product Management to the admin sidebar.

- [ ] **Step 4: Verify GREEN and build**

Run: `cd web && bun x vitest run src/features/products/__tests__/catalog.test.tsx && bun run typecheck && bun run lint && bun run build`

Expected: PASS.

### Task 5: Localize and verify

**Files:**
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/*.json`

- [ ] **Step 1: Add translation keys**

Add `Products`, `Product Management`, `Create Product`, `Edit Product`, `Manual delivery`, `Subscription`, `Product type`, `Cover image URL`, `Product summary`, `Product description`, `Publish product`, `Unpublish product`, `Contact administrator to purchase`, and `View details` to all supported locales and static key list.

- [ ] **Step 2: Verify translations and final behavior**

Run: `cd web && bun run i18n:sync && bun run typecheck && bun run lint && bun run build; cd ..; go test ./model ./controller`

Expected: PASS without missing translations, type errors, lint errors, or regression failures.

- [ ] **Step 3: Commit**

```bash
git add model controller router web/src
git commit -m "feat: add product catalog"
```
