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
import { Switch } from '@/components/ui/switch';
import { toast } from '@/components/common/Toast';
import { useTranslations } from 'next-intl';
import {
    useCreateRemoteSite,
    useUpdateRemoteSite,
    useDetectSiteType,
    SITE_TYPES,
    type RemoteSite,
    type RemoteSiteUpdateRequest,
} from '@/api/endpoints/remote-site';
import { Search } from 'lucide-react';

interface SiteDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    editingSite: RemoteSite | null;
}

export function SiteDialog({ open, onOpenChange, editingSite }: SiteDialogProps) {
    const t = useTranslations('hub');
    const createSite = useCreateRemoteSite();
    const updateSite = useUpdateRemoteSite();
    const detectType = useDetectSiteType();
    const isEditing = !!editingSite;

    const [name, setName] = useState('');
    const [baseUrl, setBaseUrl] = useState('');
    const [siteType, setSiteType] = useState('new-api');
    const [authType, setAuthType] = useState('access_token');
    const [accessToken, setAccessToken] = useState('');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [exchangeRate, setExchangeRate] = useState('7.0');
    const [enabled, setEnabled] = useState(true);

    useEffect(() => {
        if (open && editingSite) {
            setName(editingSite.name);
            setBaseUrl(editingSite.base_url);
            setSiteType(editingSite.site_type);
            setAuthType(editingSite.auth_type);
            setAccessToken('');
            setUsername(editingSite.username);
            setPassword('');
            setExchangeRate(String(editingSite.exchange_rate));
            setEnabled(editingSite.enabled);
        } else if (open && !editingSite) {
            setName('');
            setBaseUrl('');
            setSiteType('new-api');
            setAuthType('access_token');
            setAccessToken('');
            setUsername('');
            setPassword('');
            setExchangeRate('7.0');
            setEnabled(true);
        }
    }, [open, editingSite]);

    const usesPasswordAuth = siteType === 'octopus' || siteType === 'sapi';

    const handleDetect = () => {
        if (!baseUrl) return;
        detectType.mutate({ base_url: baseUrl, access_token: accessToken }, {
            onSuccess: (data) => {
                setSiteType(data.site_type);
                toast.success(`${t('detected')}: ${data.site_type}`);
            },
            onError: (err) => toast.error(err.message),
        });
    };

    const handleSubmit = () => {
        if (!name || !baseUrl || !siteType) {
            toast.error(t('requiredFields'));
            return;
        }

        if (isEditing) {
            const data: RemoteSiteUpdateRequest = { id: editingSite!.id };
            if (name !== editingSite!.name) data.name = name;
            if (baseUrl !== editingSite!.base_url) data.base_url = baseUrl;
            if (siteType !== editingSite!.site_type) data.site_type = siteType;
            if (authType !== editingSite!.auth_type) data.auth_type = authType;
            if (accessToken) data.access_token = accessToken;
            if (username !== editingSite!.username) data.username = username;
            if (password) data.password = password;
            const rate = parseFloat(exchangeRate);
            if (!isNaN(rate) && rate !== editingSite!.exchange_rate) data.exchange_rate = rate;
            if (enabled !== editingSite!.enabled) data.enabled = enabled;

            updateSite.mutate(data, {
                onSuccess: () => {
                    toast.success(t('updated'));
                    onOpenChange(false);
                },
                onError: (err) => toast.error(err.message),
            });
        } else {
            createSite.mutate({
                name,
                base_url: baseUrl,
                site_type: siteType,
                auth_type: authType,
                access_token: accessToken,
                username,
                password,
                exchange_rate: parseFloat(exchangeRate) || 7.0,
                enabled,
            }, {
                onSuccess: () => {
                    toast.success(t('created'));
                    onOpenChange(false);
                },
                onError: (err) => toast.error(err.message),
            });
        }
    };

    const isPending = createSite.isPending || updateSite.isPending;

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{isEditing ? t('editSite') : t('addSite')}</DialogTitle>
                </DialogHeader>
                <div className="grid gap-4 py-2">
                    <div className="grid gap-2">
                        <Label>{t('form.name')}</Label>
                        <Input value={name} onChange={e => setName(e.target.value)} placeholder={t('form.namePlaceholder')} />
                    </div>
                    <div className="grid gap-2">
                        <Label>{t('form.baseUrl')}</Label>
                        <div className="flex gap-2">
                            <Input
                                value={baseUrl}
                                onChange={e => setBaseUrl(e.target.value)}
                                placeholder="https://api.example.com"
                                className="flex-1"
                            />
                            <Button
                                variant="outline"
                                size="icon"
                                onClick={handleDetect}
                                disabled={detectType.isPending || !baseUrl}
                                title={t('detect')}
                            >
                                <Search className="h-4 w-4" />
                            </Button>
                        </div>
                    </div>
                    <div className="grid gap-2">
                        <Label>{t('form.siteType')}</Label>
                        <Select value={siteType} onValueChange={setSiteType}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {SITE_TYPES.map(type => (
                                    <SelectItem key={type} value={type}>{type}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    {usesPasswordAuth ? (
                        <>
                            <div className="grid gap-2">
                                <Label>{t('form.username')}</Label>
                                <Input value={username} onChange={e => setUsername(e.target.value)} />
                            </div>
                            <div className="grid gap-2">
                                <Label>{t('form.password')}</Label>
                                <Input
                                    type="password"
                                    value={password}
                                    onChange={e => setPassword(e.target.value)}
                                    placeholder={isEditing ? t('form.unchanged') : ''}
                                />
                            </div>
                        </>
                    ) : (
                        <>
                            <div className="grid gap-2">
                                <Label>{t('form.accessToken')}</Label>
                                <Input
                                    type="password"
                                    value={accessToken}
                                    onChange={e => setAccessToken(e.target.value)}
                                    placeholder={isEditing ? t('form.unchanged') : 'sk-...'}
                                />
                            </div>
                            {siteType === 'new-api' && (
                                <div className="grid gap-2">
                                    <Label>{t('form.username')}</Label>
                                    <Input
                                        value={username}
                                        onChange={e => setUsername(e.target.value)}
                                        placeholder={t('form.usernamePlaceholder')}
                                    />
                                </div>
                            )}
                        </>
                    )}
                    <div className="grid gap-2">
                        <Label>{t('form.exchangeRate')}</Label>
                        <Input
                            type="number"
                            step="0.01"
                            value={exchangeRate}
                            onChange={e => setExchangeRate(e.target.value)}
                        />
                    </div>
                    <div className="flex items-center justify-between">
                        <Label>{t('form.enabled')}</Label>
                        <Switch checked={enabled} onCheckedChange={setEnabled} />
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
