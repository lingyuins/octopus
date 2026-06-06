'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import {
    MorphingDialogClose,
    MorphingDialogDescription,
    MorphingDialogTitle,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useCreateUser } from '@/api/endpoints/user';
import { toast } from 'sonner';

const ROLE_OPTIONS = ['admin', 'editor', 'viewer'] as const;
type UserRole = (typeof ROLE_OPTIONS)[number];

export function CreateDialogContent() {
    const t = useTranslations('user');
    const { setIsOpen } = useMorphingDialog();
    const createUser = useCreateUser();
    const [formData, setFormData] = useState({
        username: '',
        password: '',
        role: 'viewer' as UserRole,
    });

    const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!formData.username.trim() || !formData.password) {
            return;
        }

        createUser.mutate({
            username: formData.username.trim(),
            password: formData.password,
            role: formData.role,
        }, {
            onSuccess: () => {
                toast.success(t('toast.created'));
                setFormData({ username: '', password: '', role: 'viewer' });
                setIsOpen(false);
            },
            onError: (error) => {
                toast.error(t('toast.createFailed'), { description: error.message });
            },
        });
    };

    return (
        <div className="w-screen max-w-full md:max-w-xl">
            <MorphingDialogTitle>
                <header className="mb-5 flex items-center justify-between">
                    <h2 className="text-xl sm:text-2xl font-bold text-card-foreground">{t('create.title')}</h2>
                    <MorphingDialogClose
                        className="relative right-0 top-0"
                        variants={{
                            initial: { opacity: 0, scale: 0.8 },
                            animate: { opacity: 1, scale: 1 },
                            exit: { opacity: 0, scale: 0.8 },
                        }}
                    />
                </header>
            </MorphingDialogTitle>
            <MorphingDialogDescription>
                <form onSubmit={handleSubmit}>
                    <FieldGroup className="gap-5 sm:gap-4">
                        <Field>
                            <FieldLabel htmlFor="user-create-username" className="text-sm sm:text-base">{t('create.username')}</FieldLabel>
                            <Input
                                id="user-create-username"
                                value={formData.username}
                                onChange={(e) => setFormData((prev) => ({ ...prev, username: e.target.value }))}
                                placeholder={t('create.usernamePlaceholder')}
                                className="rounded-xl h-11 sm:h-10"
                            />
                        </Field>

                        <Field>
                            <FieldLabel htmlFor="user-create-password" className="text-sm sm:text-base">{t('create.password')}</FieldLabel>
                            <Input
                                id="user-create-password"
                                type="password"
                                value={formData.password}
                                onChange={(e) => setFormData((prev) => ({ ...prev, password: e.target.value }))}
                                placeholder={t('create.passwordPlaceholder')}
                                className="rounded-xl h-11 sm:h-10"
                            />
                            <p className="text-xs text-muted-foreground mt-1.5">{t('create.passwordHint')}</p>
                        </Field>

                        <Field>
                            <FieldLabel htmlFor="user-create-role" className="text-sm sm:text-base">{t('create.role')}</FieldLabel>
                            <Select
                                value={formData.role}
                                onValueChange={(value) => setFormData((prev) => ({ ...prev, role: value as UserRole }))}
                            >
                                <SelectTrigger id="user-create-role" className="rounded-xl h-11 sm:h-10">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent className="rounded-xl">
                                    {ROLE_OPTIONS.map((role) => (
                                        <SelectItem key={role} value={role} className="rounded-xl py-2.5 sm:py-2">
                                            {t(`roles.${role}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </Field>

                        <Button
                            type="submit"
                            disabled={createUser.isPending || !formData.username.trim() || !formData.password}
                            className="w-full rounded-xl h-12 sm:h-11 mt-2"
                        >
                            {createUser.isPending ? t('create.submitting') : t('create.submit')}
                        </Button>
                    </FieldGroup>
                </form>
            </MorphingDialogDescription>
        </div>
    );
}
