"use client"

import { useMemo, useState } from "react"
import { motion, useReducedMotion } from "motion/react"
import { cn } from "@/lib/utils"
import { useNavStore, type NavItem } from "@/components/modules/navbar"
import { ROUTES } from "@/route/config"
import { usePreload } from "@/route/use-preload"
import { ENTRANCE_VARIANTS } from "@/lib/animations/fluid-transitions"
import { useTranslations } from "next-intl"
import { useIsMobile } from "@/hooks/use-mobile"

export function NavBar() {
    const { activeItem, orderedItems, visibleItems, setActiveItem } = useNavStore()
    const { preload } = usePreload()
    const t = useTranslations('navbar')
    const isMobile = useIsMobile()
    const reduceMotion = useReducedMotion()
    const lightweightMotion = isMobile || reduceMotion
    const [pressedItem, setPressedItem] = useState<string | null>(null)
    const visibleRouteSet = useMemo(() => new Set(visibleItems), [visibleItems])
    const routeById = useMemo(
        () => new Map(ROUTES.map((route) => [route.id as NavItem, route])),
        []
    )
    const orderedRoutes = useMemo(
        () =>
            orderedItems
                .filter((item) => visibleRouteSet.has(item))
                .map((item) => routeById.get(item))
                .filter((route) => route !== undefined),
        [orderedItems, routeById, visibleRouteSet]
    )

    return (
        <div className="relative z-50 md:min-h-full">
            <motion.nav
                aria-label={t('ariaLabel')}
                className={cn(
                    "fixed left-1/2 bottom-[calc(1.25rem+env(safe-area-inset-bottom,0px))] flex max-w-[calc(100vw-1.5rem)] -translate-x-1/2 items-center gap-1 overflow-x-auto p-2.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
                    "rounded-2xl border border-border bg-sidebar text-sidebar-foreground",
                    "md:sticky md:top-6 md:left-auto md:bottom-auto md:h-[calc(100dvh-3rem)] md:max-w-none md:translate-x-0 md:flex-col md:gap-2 md:overflow-visible md:p-3"
                )}
                variants={lightweightMotion ? undefined : ENTRANCE_VARIANTS.navbar}
                initial={lightweightMotion ? false : "initial"}
                animate={lightweightMotion ? undefined : "animate"}
            >
                <div className="pointer-events-none absolute inset-y-0 left-0 z-30 w-5 rounded-l-2xl bg-gradient-to-r from-sidebar/50 via-sidebar/20 to-transparent md:hidden" />
                <div className="pointer-events-none absolute inset-y-0 right-0 z-30 w-5 rounded-r-2xl bg-gradient-to-l from-sidebar/50 via-sidebar/20 to-transparent md:hidden" />
                {orderedRoutes.map((route, index) => {
                    const isActive = activeItem === route.id
                    return (
                        <motion.button
                            key={route.id}
                            type="button"
                            aria-label={t(route.id as NavItem)}
                            onClick={() => setActiveItem(route.id as NavItem)}
                            onMouseEnter={() => {
                                if (!isMobile) {
                                    preload(route.id)
                                }
                            }}
                            className={cn(
                                "group relative z-20 flex items-center justify-center rounded-lg border transition-[color,background-color,border-color,box-shadow] duration-150 md:w-full",
                                isMobile ? "size-9" : "h-11 md:justify-start md:gap-3 md:px-3",
                                isActive
                                    ? cn(
                                        "border-transparent bg-primary/15 text-primary border-t-2 border-t-primary md:border-t-0 md:border-l-2 md:border-l-primary md:border-r-0 md:border-b-0",
                                        isMobile && "shadow-[0_0_12px_rgba(var(--primary),0.25)]"
                                    )
                                    : "border-transparent text-sidebar-foreground/50 hover:bg-muted/60 hover:text-sidebar-foreground"
                            )}
                            initial={lightweightMotion ? false : { opacity: 0, scale: 0.8 }}
                            animate={lightweightMotion ? undefined : {
                                opacity: 1,
                                scale: 1,
                                transition: {
                                    delay: index * 0.05,
                                    duration: 0.3,
                                }
                            }}
                            whileTap={lightweightMotion
                                ? { scale: 0.88, transition: { type: "spring", stiffness: 400, damping: 17 } }
                                : { scale: 0.97 }}
                            onPointerDown={() => {
                                if (isMobile) setPressedItem(route.id)
                            }}
                            onPointerUp={() => {
                                if (isMobile) setPressedItem(null)
                            }}
                            onPointerLeave={() => {
                                if (isMobile) setPressedItem(null)
                            }}
                            onPointerCancel={() => {
                                if (isMobile) setPressedItem(null)
                            }}
                        >
                            <route.icon className="size-4 shrink-0 md:size-[1.125rem]" strokeWidth={1.5} />
                            <span className="hidden text-sm font-medium md:inline">
                                {t(route.id as NavItem)}
                            </span>
                            {isMobile && (isActive || pressedItem === route.id) && (
                                <motion.span
                                    initial={{ opacity: 0, y: 2 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    exit={{ opacity: 0 }}
                                    transition={{ duration: 0.12 }}
                                    className="pointer-events-none absolute -top-7 left-1/2 -translate-x-1/2 whitespace-nowrap rounded-md bg-popover px-1.5 py-0.5 text-[10px] font-medium text-popover-foreground shadow-md ring-1 ring-border"
                                >
                                    {t(route.id as NavItem)}
                                </motion.span>
                            )}
                        </motion.button>
                    )
                })}
            </motion.nav>
        </div>
    )
}
