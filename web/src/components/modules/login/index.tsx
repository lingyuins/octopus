'use client';

import { useState } from "react"
import { motion } from "motion/react"
import { useTranslations } from 'next-intl'
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useLogin } from "@/api/endpoints/user"
import { useAPIKeyLogin } from "@/api/endpoints/apikey"
import Logo from "@/components/modules/logo"
import { KeyRound, User } from "lucide-react"
import { ParticleBackground } from "@/components/nature"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContents,
  TabsContent,
} from "@/components/animate-ui/components/animate/tabs"

type LoginMode = 'user' | 'apikey';

export function LoginForm({ onLoginSuccess }: { onLoginSuccess?: () => void }) {
  const t = useTranslations('login')
  const [mode, setMode] = useState<LoginMode>('user')
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [apiKey, setApiKey] = useState("")
  const [error, setError] = useState<string | null>(null)
  const isMobile = useIsMobile()

  const loginMutation = useLogin()
  const apiKeyLoginMutation = useAPIKeyLogin()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    try {
      if (mode === 'user') {
        await loginMutation.mutateAsync({
          username: username.trim(),
          password,
          expire: 1440,
        })
      } else {
        await apiKeyLoginMutation.mutateAsync(apiKey)
      }

      onLoginSuccess?.()
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : t('error.generic')
      setError(errorMessage)
    }
  }

  const isPending = loginMutation.isPending || apiKeyLoginMutation.isPending

  const handleModeChange = (value: string) => {
    setMode(value as LoginMode)
    setError(null)
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
      className="relative min-h-screen min-h-dvh flex items-center justify-center px-4 sm:px-6 py-8 text-foreground overflow-hidden"
    >
      {/* Nature: 粒子背景 */}
      {!isMobile && <ParticleBackground count={40} minOpacity={0.08} maxOpacity={0.25} />}
      
      <div className="relative z-10 w-full max-w-sm">
        <div className="flex flex-col gap-6 sm:gap-8 p-5 sm:p-8 md:p-10 border-border/35 bg-card rounded-xl">
          <header className="flex flex-col items-center gap-3 sm:gap-4">
            <div className="grid size-14 sm:size-16 shrink-0 place-items-center overflow-hidden rounded-lg border-border/35 bg-card">
              <Logo size={40} />
            </div>
            <div className="flex flex-col items-center gap-1">
              <h1 className="text-2xl sm:text-3xl font-bold tracking-tight">Octopus</h1>
              <p className="text-sm text-muted-foreground/80 font-medium">{t('welcome') || 'Welcome back'}</p>
            </div>
          </header>

            <Tabs value={mode} onValueChange={handleModeChange}>
            <TabsList className="flex w-full h-auto p-1 bg-muted/50 rounded-2xl border border-border/20">
              <TabsTrigger
                value="user"
                className="rounded-xl text-xs sm:text-sm"
              >
                <User className="w-3.5 h-3.5 sm:w-4 sm:h-4" />
                <span className="truncate">{t('mode.user')}</span>
              </TabsTrigger>
              <TabsTrigger
                value="apikey"
                className="rounded-xl text-xs sm:text-sm"
              >
                <KeyRound className="w-3.5 h-3.5 sm:w-4 sm:h-4" />
                <span className="truncate">{t('mode.apikey')}</span>
              </TabsTrigger>
            </TabsList>

            <form onSubmit={handleSubmit} className="space-y-6 pt-4">
              <TabsContents>
                <TabsContent value="user" className="space-y-5">
                  <Field>
                    <FieldLabel className="text-xs font-semibold text-muted-foreground/70 ml-1" htmlFor="username">{t('username')}</FieldLabel>
                    <Input
                      id="username"
                      type="text"
                      placeholder={t('usernamePlaceholder')}
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      className="h-12 rounded-xl bg-card border-border/30"
                      autoComplete="username"
                      autoCapitalize="none"
                      autoCorrect="off"
                      spellCheck={false}
                      required={mode === 'user'}
                      disabled={isPending}
                    />
                  </Field>
                  <Field>
                    <FieldLabel className="text-xs font-semibold text-muted-foreground/70 ml-1" htmlFor="password">{t('password')}</FieldLabel>
                    <Input
                      id="password"
                      type="password"
                      placeholder={t('passwordPlaceholder')}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className="h-12 rounded-xl bg-card border-border/30"
                      autoComplete="current-password"
                      required={mode === 'user'}
                      disabled={isPending}
                    />
                  </Field>
                </TabsContent>
                <TabsContent value="apikey">
                  <Field>
                    <FieldLabel className="text-xs font-semibold text-muted-foreground/70 ml-1" htmlFor="apikey">{t('apikey')}</FieldLabel>
                    <Input
                      id="apikey"
                      type="password"
                      placeholder={t('apikeyPlaceholder')}
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      className="h-12 rounded-xl bg-card border-border/30"
                      autoComplete="off"
                      autoCapitalize="none"
                      autoCorrect="off"
                      spellCheck={false}
                      required={mode === 'apikey'}
                      disabled={isPending}
                    />
                  </Field>
                </TabsContent>
              </TabsContents>

              {error && (
                <motion.div
                  initial={{ opacity: 0, y: -5 }}
                  animate={{ opacity: 1, y: 0 }}
                  className="px-1"
                >
                  <FieldDescription className="text-destructive font-medium text-xs bg-destructive/5 p-2 rounded-lg border border-destructive/10">
                    {error}
                  </FieldDescription>
                </motion.div>
              )}

              <Button
                type="submit"
                disabled={isPending}
                className="w-full h-12 rounded-xl bg-primary text-primary-foreground hover:bg-primary/90 transition-all active:scale-[0.98]"
              >
                {isPending ? t('button.loading') : t('button.submit')}
              </Button>
            </form>
          </Tabs>
        </div>
      </div>
    </motion.div>
  )
}
