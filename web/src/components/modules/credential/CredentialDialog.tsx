'use client';

import { useEffect, useState } from 'react';
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from '@/components/common/Toast';
import { useTranslations } from 'next-intl';
import {
    useCreateCredential,
    useUpdateCredential,
    API_TYPES,
    type APICredentialProfile,
    type APICredentialUpdateRequest,
} from '@/api/endpoints/api-credential';

interface CredentialDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    editing: APICredentialProfile | null;
}

export function CredentialDialog({ open, onOpenChange, editing }: CredentialDialogProps) {
    const t = useTranslations('credential');
    const createMut = useCreateCredential();
    const updateMut = useUpdateCredential();
    const isEditing = !!editing;

    const [name, setName] = useState('');
    const [apiType, setApiType] = useState('openai');
    const [baseUrl, setBaseUrl] = useState('');
    const [apiKey, setApiKey] = useState('');
    const [notes, setNotes] = useState('');

    useEffect(() => {
        if (open && editing) {
            setName(editing.name);
            setApiType(editing.api_type);
            setBaseUrl(editing.base_url);
            setApiKey('');
            setNotes(editing.notes);
        } else if (open) {
            setName('');
            setApiType('openai');
            setBaseUrl('');
            setApiKey('');
            setNotes('');
        }
    }, [open, editing]);

    const handleSubmit = () => {
        if (!name || !baseUrl) {
            toast.error(t('requiredFields'));
            return;
        }

        if (isEditing) {
            const data: APICredentialUpdateRequest = { id: editing!.id };
            if (name !== editing!.name) data.name = name;
            if (apiType !== editing!.api_type) data.api_type = apiType;
            if (baseUrl !== editing!.base_url) data.base_url = baseUrl;
            if (apiKey) data.api_key = apiKey;
            if (notes !== editing!.notes) data.notes = notes;

            updateMut.mutate(data, {
                onSuccess: () => { toast.success(t('updated')); onOpenChange(false); },
                onError: (err) => toast.error(err.message),
            });
        } else {
            if (!apiKey) {
                toast.error(t('requiredFields'));
                return;
            }
            createMut.mutate({ name, api_type: apiType, base_url: baseUrl, api_key: apiKey, notes }, {
                onSuccess: () => { toast.success(t('created')); onOpenChange(false); },
                onError: (err) => toast.error(err.message),
            });
        }
    };

    const isPending = createMut.isPending || updateMut.isPending;

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>{isEditing ? t('edit') : t('add')}</DialogTitle>
                </DialogHeader>
                <div className="grid gap-4 py-2">
                    <div className="grid gap-2">
                        <Label>{t('form.name')}</Label>
                        <Input value={name} onChange={e => setName(e.target.value)} placeholder={t('form.namePlaceholder')} />
                    </div>
                    <div className="grid gap-2">
                        <Label>{t('form.apiType')}</Label>
                        <Select value={apiType} onValueChange={setApiType}>
                            <SelectTrigger><SelectValue /></SelectTrigger>
                            <SelectContent>
                                {API_TYPES.map(t => <SelectItem key={t} value={t}>{t}</SelectItem>)}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="grid gap-2">
                        <Label>{t('form.baseUrl')}</Label>
                        <Input value={baseUrl} onChange={e => setBaseUrl(e.target.value)} placeholder="https://api.openai.com" />
                    </div>
                    <div className="grid gap-2">
                        <Label>{t('form.apiKey')}</Label>
                        <Input
                            type="password"
                            value={apiKey}
                            onChange={e => setApiKey(e.target.value)}
                            placeholder={isEditing ? t('form.unchanged') : 'sk-...'}
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label>{t('form.notes')}</Label>
                        <Input value={notes} onChange={e => setNotes(e.target.value)} />
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>{t('cancel')}</Button>
                    <Button onClick={handleSubmit} disabled={isPending}>
                        {isPending ? t('saving') : (isEditing ? t('save') : t('create'))}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
