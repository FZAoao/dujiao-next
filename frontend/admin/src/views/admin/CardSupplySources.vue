<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminCardSupplySource, AdminProduct, AdminProductSKU } from '@/api/types'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { confirmAction } from '@/utils/confirm'
import { copyText } from '@/utils/clipboard'
import { notifyError, notifySuccess } from '@/utils/notify'
import { Copy, KeyRound, Loader2, Pencil, Plus, RotateCcw, Trash2 } from 'lucide-vue-next'

const { t, locale } = useI18n()

type SourceForm = {
  name: string
  description: string
  product_id: number
  sku_id: number
  max_batch_size: number
  ip_allowlist: string
}

type RevealedCredentials = {
  api_key: string
  api_secret: string
}

const emptyForm = (): SourceForm => ({
  name: '',
  description: '',
  product_id: 0,
  sku_id: 0,
  max_batch_size: 500,
  ip_allowlist: '',
})

const loading = ref(true)
const productsLoading = ref(true)
const saving = ref(false)
const sources = ref<AdminCardSupplySource[]>([])
const products = ref<AdminProduct[]>([])
const showFormDialog = ref(false)
const editingSource = ref<AdminCardSupplySource | null>(null)
const showCredentialsDialog = ref(false)
const revealedCredentials = ref<RevealedCredentials | null>(null)
const form = reactive<SourceForm>(emptyForm())

const sourceTitle = computed(() => editingSource.value ? t('admin.cardSupplySources.editTitle') : t('admin.cardSupplySources.createTitle'))

const activeProducts = computed(() => products.value.filter((product) =>
  Array.isArray(product.skus) && product.skus.some((sku) => sku.is_active),
))

const selectedProduct = computed(() => products.value.find((product) => product.id === form.product_id))
const availableSkus = computed<AdminProductSKU[]>(() =>
  (selectedProduct.value?.skus || []).filter((sku) => sku.is_active),
)

const productLabel = (product: AdminProduct) => {
  const title = product.title || {}
  return title[locale.value] || title['zh-CN'] || title['en-US'] || product.slug || `#${product.id}`
}

const skuLabel = (sku: AdminProductSKU) => {
  const spec = Object.values(sku.spec_values || {}).filter(Boolean).join(' / ')
  return spec ? `${sku.sku_code || `#${sku.id}`} · ${spec}` : (sku.sku_code || `#${sku.id}`)
}

const sourceProductLabel = (source: AdminCardSupplySource) => {
  const product = products.value.find((item) => item.id === source.product_id)
  return product ? productLabel(product) : `#${source.product_id}`
}

const sourceSkuLabel = (source: AdminCardSupplySource) => {
  const product = products.value.find((item) => item.id === source.product_id)
  const sku = product?.skus?.find((item) => item.id === source.sku_id)
  return sku ? skuLabel(sku) : `#${source.sku_id}`
}

const formatDate = (value?: string) => value ? new Date(value).toLocaleString() : t('admin.cardSupplySources.never')

const errorMessage = (error: unknown) => {
  const value = error as { __notified?: boolean; message?: string } | null
  if (!value?.__notified) notifyError(value?.message || t('admin.cardSupplySources.requestFailed'))
}

const fetchSources = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getCardSupplySources()
    sources.value = Array.isArray(res.data?.data) ? res.data.data : []
  } catch (error) {
    sources.value = []
    errorMessage(error)
  } finally {
    loading.value = false
  }
}

const fetchProducts = async () => {
  productsLoading.value = true
  try {
    const allProducts: AdminProduct[] = []
    const pageSize = 200
    let page = 1
    let totalPage = 1

    do {
      const res = await adminAPI.getProducts({ page, page_size: pageSize, is_active: true })
      const pageProducts = Array.isArray(res.data?.data) ? res.data.data as AdminProduct[] : []
      allProducts.push(...pageProducts)
      totalPage = Math.max(1, Number(res.data?.pagination?.total_page || 1))
      page += 1
    } while (page <= totalPage)

    products.value = allProducts
  } catch (error) {
    products.value = []
    errorMessage(error)
  } finally {
    productsLoading.value = false
  }
}

watch(() => form.product_id, () => {
  if (!availableSkus.value.some((sku) => sku.id === form.sku_id)) {
    form.sku_id = 0
  }
})

const resetForm = () => Object.assign(form, emptyForm())

const openCreateDialog = () => {
  editingSource.value = null
  resetForm()
  showFormDialog.value = true
}

const openEditDialog = (source: AdminCardSupplySource) => {
  editingSource.value = source
  Object.assign(form, {
    name: source.name,
    description: source.description || '',
    product_id: source.product_id,
    sku_id: source.sku_id,
    max_batch_size: source.max_batch_size || 500,
    ip_allowlist: source.ip_allowlist || '',
  })
  showFormDialog.value = true
}

const closeFormDialog = () => {
  if (!saving.value) showFormDialog.value = false
}

const handleSubmit = async () => {
  if (!form.name.trim() || !form.product_id || !form.sku_id || form.max_batch_size < 1) return
  saving.value = true
  try {
    const payload = {
      name: form.name.trim(),
      description: form.description.trim(),
      product_id: Number(form.product_id),
      sku_id: Number(form.sku_id),
      max_batch_size: Number(form.max_batch_size),
      ip_allowlist: form.ip_allowlist.trim(),
    }
    if (editingSource.value) {
      await adminAPI.updateCardSupplySource(editingSource.value.id, payload)
      notifySuccess(t('admin.cardSupplySources.updateSuccess'))
    } else {
      const res = await adminAPI.createCardSupplySource(payload)
      const data = res.data?.data as Partial<RevealedCredentials> | undefined
      if (data?.api_key && data.api_secret) {
        revealedCredentials.value = { api_key: data.api_key, api_secret: data.api_secret }
        showCredentialsDialog.value = true
      }
      notifySuccess(t('admin.cardSupplySources.createSuccess'))
    }
    showFormDialog.value = false
    await fetchSources()
  } catch (error) {
    errorMessage(error)
  } finally {
    saving.value = false
  }
}

const handleToggleStatus = async (source: AdminCardSupplySource) => {
  const nextStatus = source.status === 1 ? 0 : 1
  const message = nextStatus === 1
    ? t('admin.cardSupplySources.enableConfirm', { name: source.name })
    : t('admin.cardSupplySources.disableConfirm', { name: source.name })
  if (!await confirmAction(message)) return
  try {
    await adminAPI.updateCardSupplySourceStatus(source.id, { status: nextStatus })
    notifySuccess(t('admin.cardSupplySources.statusSuccess'))
    await fetchSources()
  } catch (error) {
    errorMessage(error)
  }
}

const handleResetSecret = async (source: AdminCardSupplySource) => {
  if (!await confirmAction(t('admin.cardSupplySources.resetConfirm', { name: source.name }))) return
  try {
    const res = await adminAPI.resetCardSupplySourceSecret(source.id)
    const data = res.data?.data as Partial<RevealedCredentials> | undefined
    if (data?.api_key && data.api_secret) {
      revealedCredentials.value = { api_key: data.api_key, api_secret: data.api_secret }
      showCredentialsDialog.value = true
    }
    notifySuccess(t('admin.cardSupplySources.resetSuccess'))
    await fetchSources()
  } catch (error) {
    errorMessage(error)
  }
}

const handleDelete = async (source: AdminCardSupplySource) => {
  if (!await confirmAction(t('admin.cardSupplySources.deleteConfirm', { name: source.name }))) return
  try {
    await adminAPI.deleteCardSupplySource(source.id)
    notifySuccess(t('admin.cardSupplySources.deleteSuccess'))
    await fetchSources()
  } catch (error) {
    errorMessage(error)
  }
}

const copyCredential = async (value: string) => {
  try {
    await copyText(value)
    notifySuccess(t('admin.cardSupplySources.copied'))
  } catch (error) {
    errorMessage(error)
  }
}

const closeCredentialsDialog = () => {
  showCredentialsDialog.value = false
  revealedCredentials.value = null
}

onMounted(() => {
  void Promise.all([fetchSources(), fetchProducts()])
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-2xl font-bold tracking-tight">{{ t('admin.cardSupplySources.title') }}</h2>
        <p class="text-muted-foreground">{{ t('admin.cardSupplySources.subtitle') }}</p>
      </div>
      <Button size="sm" class="w-full sm:w-auto" @click="openCreateDialog">
        <Plus class="mr-2 h-4 w-4" />
        {{ t('admin.cardSupplySources.create') }}
      </Button>
    </div>

    <Card>
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-base">
          <KeyRound class="h-4 w-4" />
          {{ t('admin.cardSupplySources.title') }}
        </CardTitle>
        <CardDescription>{{ t('admin.cardSupplySources.tableHint') }}</CardDescription>
      </CardHeader>
      <CardContent class="overflow-x-auto p-0">
        <Table class="min-w-[1120px]">
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead class="min-w-[170px]">{{ t('admin.cardSupplySources.name') }}</TableHead>
              <TableHead class="min-w-[220px]">{{ t('admin.cardSupplySources.binding') }}</TableHead>
              <TableHead class="min-w-[190px]">{{ t('admin.cardSupplySources.apiKey') }}</TableHead>
              <TableHead class="min-w-[120px]">{{ t('admin.cardSupplySources.status') }}</TableHead>
              <TableHead class="min-w-[190px]">{{ t('admin.cardSupplySources.lastCall') }}</TableHead>
              <TableHead class="min-w-[240px]">{{ t('admin.cardSupplySources.actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-if="loading">
              <TableCell :colspan="7" class="py-10 text-center text-muted-foreground">
                <Loader2 class="mx-auto h-5 w-5 animate-spin" />
              </TableCell>
            </TableRow>
            <TableRow v-else-if="sources.length === 0">
              <TableCell :colspan="7" class="py-10 text-center text-muted-foreground">
                {{ t('admin.cardSupplySources.empty') }}
              </TableCell>
            </TableRow>
            <template v-else>
              <TableRow v-for="source in sources" :key="source.id">
                <TableCell>{{ source.id }}</TableCell>
                <TableCell>
                  <div class="font-medium">{{ source.name }}</div>
                  <div v-if="source.description" class="mt-1 max-w-[220px] break-words text-xs text-muted-foreground">
                    {{ source.description }}
                  </div>
                </TableCell>
                <TableCell>
                  <div class="font-medium">{{ sourceProductLabel(source) }}</div>
                  <div class="mt-1 break-all text-xs text-muted-foreground">{{ sourceSkuLabel(source) }}</div>
                </TableCell>
                <TableCell>
                  <div class="flex items-center gap-1">
                    <code class="max-w-[170px] break-all rounded bg-muted px-1.5 py-0.5 text-xs">{{ source.api_key }}</code>
                    <Button variant="ghost" size="sm" class="h-7 w-7 shrink-0 p-0" @click="copyCredential(source.api_key)">
                      <Copy class="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </TableCell>
                <TableCell>
                  <Badge :variant="source.status === 1 ? 'default' : 'secondary'">
                    {{ source.status === 1 ? t('admin.cardSupplySources.active') : t('admin.cardSupplySources.disabled') }}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div>{{ formatDate(source.last_used_at) }}</div>
                  <div v-if="source.last_error_code" class="mt-1 text-xs text-destructive">
                    {{ source.last_error_code }}
                  </div>
                </TableCell>
                <TableCell>
                  <div class="flex flex-wrap gap-1.5">
                    <Button variant="outline" size="sm" @click="openEditDialog(source)">
                      <Pencil class="mr-1.5 h-3.5 w-3.5" />
                      {{ t('admin.cardSupplySources.edit') }}
                    </Button>
                    <Button variant="outline" size="sm" @click="handleToggleStatus(source)">
                      {{ source.status === 1 ? t('admin.cardSupplySources.disable') : t('admin.cardSupplySources.enable') }}
                    </Button>
                    <Button variant="outline" size="sm" @click="handleResetSecret(source)">
                      <RotateCcw class="mr-1.5 h-3.5 w-3.5" />
                      {{ t('admin.cardSupplySources.resetSecret') }}
                    </Button>
                    <Button variant="destructive" size="sm" @click="handleDelete(source)">
                      <Trash2 class="mr-1.5 h-3.5 w-3.5" />
                      {{ t('admin.cardSupplySources.delete') }}
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            </template>
          </TableBody>
        </Table>
      </CardContent>
    </Card>

    <Dialog v-model:open="showFormDialog">
      <DialogContent class="w-[calc(100vw-1rem)] max-w-2xl p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{{ sourceTitle }}</DialogTitle>
          <DialogDescription>{{ t('admin.cardSupplySources.formDescription') }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="space-y-2">
            <Label for="card-supply-source-name">{{ t('admin.cardSupplySources.name') }}</Label>
            <Input id="card-supply-source-name" v-model="form.name" :placeholder="t('admin.cardSupplySources.namePlaceholder')" maxlength="100" />
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-2">
              <Label for="card-supply-source-product">{{ t('admin.cardSupplySources.product') }}</Label>
              <select
                id="card-supply-source-product"
                v-model.number="form.product_id"
                class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring"
                :disabled="productsLoading"
              >
                <option :value="0">{{ productsLoading ? t('admin.cardSupplySources.loading') : t('admin.cardSupplySources.selectProduct') }}</option>
                <option v-for="product in activeProducts" :key="product.id" :value="product.id">{{ productLabel(product) }}</option>
              </select>
              <p v-if="!productsLoading && activeProducts.length === 0" class="text-xs text-destructive">{{ t('admin.cardSupplySources.noProducts') }}</p>
            </div>
            <div class="space-y-2">
              <Label for="card-supply-source-sku">{{ t('admin.cardSupplySources.sku') }}</Label>
              <select
                id="card-supply-source-sku"
                v-model.number="form.sku_id"
                class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring"
                :disabled="!form.product_id || availableSkus.length === 0"
              >
                <option :value="0">{{ t('admin.cardSupplySources.selectSku') }}</option>
                <option v-for="sku in availableSkus" :key="sku.id" :value="sku.id">{{ skuLabel(sku) }}</option>
              </select>
              <p v-if="form.product_id && availableSkus.length === 0" class="text-xs text-destructive">{{ t('admin.cardSupplySources.noSkus') }}</p>
            </div>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-2">
              <Label for="card-supply-source-max-batch">{{ t('admin.cardSupplySources.maxBatchSize') }}</Label>
              <Input id="card-supply-source-max-batch" v-model.number="form.max_batch_size" type="number" min="1" max="5000" />
              <p class="text-xs text-muted-foreground">{{ t('admin.cardSupplySources.maxBatchHint') }}</p>
            </div>
            <div class="space-y-2">
              <Label for="card-supply-source-ip-allowlist">{{ t('admin.cardSupplySources.ipAllowlist') }}</Label>
              <Textarea id="card-supply-source-ip-allowlist" v-model="form.ip_allowlist" rows="2" :placeholder="t('admin.cardSupplySources.ipAllowlistPlaceholder')" />
              <p class="text-xs text-muted-foreground">{{ t('admin.cardSupplySources.ipAllowlistHint') }}</p>
            </div>
          </div>
          <div class="space-y-2">
            <Label for="card-supply-source-description">{{ t('admin.cardSupplySources.description') }}</Label>
            <Textarea id="card-supply-source-description" v-model="form.description" rows="3" maxlength="500" :placeholder="t('admin.cardSupplySources.descriptionPlaceholder')" />
          </div>
        </div>
        <DialogFooter class="flex-col-reverse sm:flex-row">
          <Button class="w-full sm:w-auto" variant="outline" :disabled="saving" @click="closeFormDialog">{{ t('admin.cardSupplySources.cancel') }}</Button>
          <Button
            class="w-full sm:w-auto"
            :disabled="saving || !form.name.trim() || !form.product_id || !form.sku_id || !form.max_batch_size"
            @click="handleSubmit"
          >
            <Loader2 v-if="saving" class="mr-2 h-4 w-4 animate-spin" />
            {{ t('admin.cardSupplySources.save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :open="showCredentialsDialog" @update:open="(value: boolean) => { if (!value) closeCredentialsDialog() }">
      <DialogContent class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{{ t('admin.cardSupplySources.credentialsTitle') }}</DialogTitle>
          <DialogDescription>{{ t('admin.cardSupplySources.credentialsDescription') }}</DialogDescription>
        </DialogHeader>
        <div v-if="revealedCredentials" class="space-y-4">
          <div class="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-100">
            {{ t('admin.cardSupplySources.secretWarning') }}
          </div>
          <div class="space-y-2">
            <Label>{{ t('admin.cardSupplySources.apiKey') }}</Label>
            <div class="flex gap-2">
              <Input :model-value="revealedCredentials.api_key" readonly class="font-mono text-xs" />
              <Button variant="outline" size="icon" :aria-label="t('admin.cardSupplySources.copy')" @click="copyCredential(revealedCredentials.api_key)"><Copy class="h-4 w-4" /></Button>
            </div>
          </div>
          <div class="space-y-2">
            <Label>{{ t('admin.cardSupplySources.apiSecret') }}</Label>
            <div class="flex gap-2">
              <Input :model-value="revealedCredentials.api_secret" readonly class="font-mono text-xs" />
              <Button variant="outline" size="icon" :aria-label="t('admin.cardSupplySources.copy')" @click="copyCredential(revealedCredentials.api_secret)"><Copy class="h-4 w-4" /></Button>
            </div>
          </div>
          <p class="text-xs text-muted-foreground">{{ t('admin.cardSupplySources.secretOnce') }}</p>
        </div>
        <DialogFooter>
          <Button @click="closeCredentialsDialog">{{ t('admin.cardSupplySources.close') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
