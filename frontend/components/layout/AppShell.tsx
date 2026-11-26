"use client";

import { useCallback, useMemo, useState, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import { usePermissions } from "@/lib/permissions";
import { organizationDisplayName, organizationLogoUrl } from "@/lib/organization";
import {
  boxNavigation,
  cloudEntries,
  cloudNavigation,
  currentBoxTab,
  currentEntry,
} from "@/lib/navigation/shell";
import BoxTabs from "./BoxTabs";
import ConnectivityBadge from "./ConnectivityBadge";
import Sidebar from "./Sidebar";
import TopBar from "./TopBar";
import UserMenu from "./UserMenu";
import styles from "./AppShell.module.css";

export default function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const { organization, hasPermission, isEdge, isAdmin } = usePermissions();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const closeDrawer = useCallback(() => setDrawerOpen(false), []);

  const context = useMemo(
    () => ({ hasPermission, plan: organization?.plan, isEdge, isAdmin }),
    [hasPermission, organization?.plan, isEdge, isAdmin]
  );
  const brandName = organization ? organizationDisplayName(organization) : "Kuenta";
  const logoUrl = organization ? organizationLogoUrl(organization) : null;

  if (isEdge) {
    const navigation = boxNavigation(context);
    return (
      <div className={styles.box}>
        <BoxTabs navigation={navigation} current={currentBoxTab(pathname, navigation)} logoUrl={logoUrl}>
          <ConnectivityBadge />
          <UserMenu />
        </BoxTabs>
        <main className={styles.content}>{children}</main>
      </div>
    );
  }

  const navigation = cloudNavigation(context);
  const current = currentEntry(pathname, cloudEntries(navigation));
  return (
    <div className={styles.cloud}>
      <Sidebar
        navigation={navigation}
        current={current}
        brandName={brandName}
        logoUrl={logoUrl}
        open={drawerOpen}
        onClose={closeDrawer}
      />
      <div className={styles.main}>
        <TopBar title={current?.title ?? brandName} menuOpen={drawerOpen} onMenu={() => setDrawerOpen(true)}>
          <ConnectivityBadge />
          <UserMenu />
        </TopBar>
        <main className={styles.content}>{children}</main>
      </div>
    </div>
  );
}
