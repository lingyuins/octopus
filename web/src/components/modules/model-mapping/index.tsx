'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslations } from 'next-intl'
import {
  listModelMappings,
  createModelMapping,
  updateModelMapping,
  deleteModelMapping,
  type ModelMapping,
  type CreateModelMappingRequest,
  type UpdateModelMappingRequest,
} from '@/api/endpoints/model-mapping'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Plus, Pencil, Trash2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'

export function ModelMappingPage() {
  const t = useTranslations('model_mapping')
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingMapping, setEditingMapping] = useState<ModelMapping | null>(null)
  const [formData, setFormData] = useState<CreateModelMappingRequest | UpdateModelMappingRequest>({
    name: '',
    pattern: '',
    match_type: 'exact',
    target_model: '',
    priority: 0,
    enabled: true,
    scope_group_id: null,
  })

  const { data: mappings, isLoading } = useQuery({
    queryKey: ['model-mappings'],
    queryFn: listModelMappings,
  })

  const createMutation = useMutation({
    mutationFn: createModelMapping,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-mappings'] })
      toast.success(t('create_success'))
      setDialogOpen(false)
      resetForm()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('create_error'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateModelMappingRequest }) =>
      updateModelMapping(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-mappings'] })
      toast.success(t('update_success'))
      setDialogOpen(false)
      resetForm()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('update_error'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteModelMapping,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-mappings'] })
      toast.success(t('delete_success'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('delete_error'))
    },
  })

  const resetForm = () => {
    setEditingMapping(null)
    setFormData({
      name: '',
      pattern: '',
      match_type: 'exact',
      target_model: '',
      priority: 0,
      enabled: true,
      scope_group_id: null,
    })
  }

  const handleCreate = () => {
    setEditingMapping(null)
    resetForm()
    setDialogOpen(true)
  }

  const handleEdit = (mapping: ModelMapping) => {
    setEditingMapping(mapping)
    setFormData({
      name: mapping.name,
      pattern: mapping.pattern,
      match_type: mapping.match_type,
      target_model: mapping.target_model,
      priority: mapping.priority,
      enabled: mapping.enabled,
      scope_group_id: mapping.scope_group_id,
    })
    setDialogOpen(true)
  }

  const handleDelete = (id: number) => {
    if (confirm(t('confirm_delete'))) {
      deleteMutation.mutate(id)
    }
  }

  const handleSubmit = () => {
    if (editingMapping) {
      updateMutation.mutate({ id: editingMapping.id, data: formData })
    } else {
      createMutation.mutate(formData as CreateModelMappingRequest)
    }
  }

  const getMatchTypeBadge = (type: string) => {
    switch (type) {
      case 'exact':
        return <Badge variant="default">{t('match_exact')}</Badge>
      case 'wildcard':
        return <Badge variant="secondary">{t('match_wildcard')}</Badge>
      case 'regex':
        return <Badge variant="outline">{t('match_regex')}</Badge>
      default:
        return <Badge>{type}</Badge>
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <RefreshCw className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="container mx-auto py-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-3xl font-bold">{t('title')}</h1>
          <p className="text-muted-foreground mt-1">{t('description')}</p>
        </div>
        <Button onClick={handleCreate}>
          <Plus className="w-4 h-4 mr-2" />
          {t('add')}
        </Button>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('name')}</TableHead>
              <TableHead>{t('pattern')}</TableHead>
              <TableHead>{t('match_type')}</TableHead>
              <TableHead>{t('target_model')}</TableHead>
              <TableHead>{t('priority')}</TableHead>
              <TableHead>{t('scope')}</TableHead>
              <TableHead>{t('status')}</TableHead>
              <TableHead className="text-right">{t('actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {mappings && mappings.length > 0 ? (
              mappings.map((mapping) => (
                <TableRow key={mapping.id}>
                  <TableCell className="font-medium">{mapping.name}</TableCell>
                  <TableCell className="font-mono text-sm">{mapping.pattern}</TableCell>
                  <TableCell>{getMatchTypeBadge(mapping.match_type)}</TableCell>
                  <TableCell className="font-mono text-sm">{mapping.target_model}</TableCell>
                  <TableCell>{mapping.priority}</TableCell>
                  <TableCell>
                    {mapping.scope_group_id ? (
                      <span>Group {mapping.scope_group_id}</span>
                    ) : (
                      <span className="text-muted-foreground">{t('global')}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    {mapping.enabled ? (
                      <Badge variant="default">{t('enabled')}</Badge>
                    ) : (
                      <Badge variant="secondary">{t('disabled')}</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleEdit(mapping)}
                      >
                        <Pencil className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDelete(mapping.id)}
                      >
                        <Trash2 className="w-4 h-4 text-destructive" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
                  {t('empty')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editingMapping ? t('edit_title') : t('create_title')}
            </DialogTitle>
            <DialogDescription>
              {editingMapping ? t('edit_description') : t('create_description')}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="name">{t('name')} *</Label>
              <Input
                id="name"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder={t('name_placeholder')}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="pattern">{t('pattern')} *</Label>
              <Input
                id="pattern"
                value={formData.pattern}
                onChange={(e) => setFormData({ ...formData, pattern: e.target.value })}
                placeholder={t('pattern_placeholder')}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="match_type">{t('match_type')} *</Label>
              <Select
                value={formData.match_type}
                onValueChange={(value) => setFormData({ ...formData, match_type: value as "exact" | "wildcard" | "regex" })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="exact">{t('match_exact')}</SelectItem>
                  <SelectItem value="wildcard">{t('match_wildcard')}</SelectItem>
                  <SelectItem value="regex">{t('match_regex')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="target_model">{t('target_model')} *</Label>
              <Input
                id="target_model"
                value={formData.target_model}
                onChange={(e) => setFormData({ ...formData, target_model: e.target.value })}
                placeholder={t('target_model_placeholder')}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="priority">{t('priority')}</Label>
              <Input
                id="priority"
                type="number"
                value={formData.priority || 0}
                onChange={(e) => setFormData({ ...formData, priority: parseInt(e.target.value) })}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="scope_group_id">{t('scope_group_id')}</Label>
              <Input
                id="scope_group_id"
                type="number"
                value={formData.scope_group_id || ''}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    scope_group_id: e.target.value ? parseInt(e.target.value) : null,
                  })
                }
                placeholder={t('scope_group_id_placeholder')}
              />
            </div>

            <div className="flex items-center space-x-2">
              <Switch
                id="enabled"
                checked={formData.enabled}
                onCheckedChange={(checked) => setFormData({ ...formData, enabled: checked })}
              />
              <Label htmlFor="enabled">{t('enabled')}</Label>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              {t('cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={createMutation.isPending || updateMutation.isPending}>
              {editingMapping ? t('update') : t('create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
