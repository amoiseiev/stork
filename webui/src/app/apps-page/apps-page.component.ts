import { Component, OnDestroy, OnInit } from '@angular/core'
import { ActivatedRoute, Router } from '@angular/router'
import { BehaviorSubject, Subscription } from 'rxjs'

import { MessageService, MenuItem } from 'primeng/api'

import { daemonStatusErred, getErrorMessage } from '../utils'
import { ServicesService } from '../backend/api/api'
import { LoadingService } from '../loading.service'

function htmlizeExtVersion(app) {
    if (app.details.extendedVersion) {
        app.details.extendedVersion = app.details.extendedVersion.replace(/\n/g, '<br>')
    }
    if (app.details.daemons) {
        for (const d of app.details.daemons) {
            if (d.extendedVersion) {
                d.extendedVersion = d.extendedVersion.replace(/\n/g, '<br>')
            }
        }
    }
}

/**
 * Sets boolean flag indicating if there are communication errors with
 * daemons belonging to the app.
 *
 * @param app app for which the communication status with the daemons
 *            should be updated.
 */
function setDaemonStatusErred(app) {
    if (app.details.daemons) {
        for (const d of app.details.daemons) {
            if (d.active && daemonStatusErred(d)) {
                d.statusErred = true
            } else {
                d.statusErred = false
            }
        }
    }
}

@Component({
    selector: 'app-apps-page',
    templateUrl: './apps-page.component.html',
    styleUrls: ['./apps-page.component.sass'],
})
export class AppsPageComponent implements OnInit, OnDestroy {
    private subscriptions = new Subscription()
    breadcrumbs = [{ label: 'Services' }, { label: 'Apps' }]

    appType = ''
    // apps table
    apps: any[]
    totalApps: number
    appMenuItems: MenuItem[]

    // action panel
    filterText = ''

    // app tabs
    activeTabIdx = 0
    tabs: MenuItem[]
    activeItem: MenuItem
    openedApps: any
    appTab: any = null

    refreshedAppTab = new BehaviorSubject(this.appTab)

    constructor(
        private route: ActivatedRoute,
        private router: Router,
        private servicesApi: ServicesService,
        private msgSrv: MessageService,
        private loadingService: LoadingService
    ) {}

    ngOnDestroy(): void {
        this.subscriptions.unsubscribe()
    }

    getAppsLabel() {
        if (this.appType === 'bind9') {
            return 'BIND 9 Apps'
        } else {
            return 'Kea Apps'
        }
    }

    switchToTab(index) {
        if (this.activeTabIdx === index) {
            return
        }
        this.activeTabIdx = index
        this.activeItem = this.tabs[index]

        if (index > 0) {
            this.appTab = this.openedApps[index - 1]
        }
    }

    addAppTab(app) {
        this.openedApps.push({
            app,
        })
        this.tabs.push({
            label: `${app.name}`,
            routerLink: '/apps/' + this.appType + '/' + app.id,
        })
    }

    ngOnInit() {
        this.subscriptions.add(
            this.route.paramMap.subscribe((params) => {
                const newAppType = params.get('appType')

                if (newAppType !== this.appType) {
                    this.appType = newAppType
                    this.breadcrumbs[1]['label'] = this.getAppsLabel()

                    this.tabs = [{ label: 'All', routerLink: '/apps/' + this.appType + '/all' }]

                    this.apps = []
                    this.appMenuItems = [
                        {
                            label: 'Refresh',
                            id: 'refresh-single-app',
                            icon: 'pi pi-refresh',
                        },
                    ]

                    this.openedApps = []

                    this.loadApps({ first: 0, rows: 10 })
                }

                const appIdStr = params.get('id')
                if (appIdStr === 'all') {
                    this.switchToTab(0)
                } else {
                    const appId = parseInt(appIdStr, 10)

                    let found = false
                    // if tab for this app is already opened then switch to it
                    for (let idx = 0; idx < this.openedApps.length; idx++) {
                        const s = this.openedApps[idx].app
                        if (s.id === appId) {
                            this.switchToTab(idx + 1)
                            found = true
                        }
                    }

                    // if tab is not opened then search for list of apps if the one is present there,
                    // if so then open it in new tab and switch to it
                    if (!found) {
                        for (const s of this.apps) {
                            if (s.id === appId) {
                                this.addAppTab(s)
                                this.switchToTab(this.tabs.length - 1)
                                found = true
                                break
                            }
                        }
                    }

                    // if app is not loaded in list fetch it individually
                    if (!found) {
                        this.servicesApi
                            .getApp(appId)
                            .toPromise()
                            .then((data) => {
                                if (data.type !== this.appType) {
                                    this.msgSrv.add({
                                        severity: 'error',
                                        summary: 'Cannot find app',
                                        detail: 'Cannot find app with ID ' + appId,
                                        life: 10000,
                                    })
                                    this.router.navigate(['/apps/' + this.appType + '/all'])
                                    return
                                }

                                htmlizeExtVersion(data)
                                setDaemonStatusErred(data)
                                this.addAppTab(data)
                                this.switchToTab(this.tabs.length - 1)
                            })
                            .catch((err) => {
                                let msg = getErrorMessage(err)
                                this.msgSrv.add({
                                    severity: 'error',
                                    summary: 'Cannot get app',
                                    detail: 'Getting app with ID ' + appId + ' erred: ' + msg,
                                    life: 10000,
                                })
                                this.router.navigate(['/apps/' + this.appType + '/all'])
                            })
                    }
                }
            })
        )
    }

    loadApps(event) {
        if (this.appType === '') {
            // appType has not been set yet so do not load anything
            return
        }
        let text
        if (event.filters && event.filters.text) {
            text = event.filters.text.value
        }

        // ToDo: Uncaught promise
        // If any HTTP exception will be thrown then the promise
        // fails, but a user doesn't get any message, popup, log.
        this.servicesApi
            .getApps(event.first, event.rows, text, this.appType)
            .toPromise()
            .then((data) => {
                this.apps = data.items
                this.totalApps = data.total
                for (const s of this.apps) {
                    htmlizeExtVersion(s)
                    setDaemonStatusErred(s)
                }
            })
    }

    keyUpFilterText(appsTable, event) {
        if (this.filterText.length >= 3 || event.key === 'Enter') {
            appsTable.filter(this.filterText, 'text', 'equals')
        }
    }

    closeTab(event, idx) {
        this.openedApps.splice(idx - 1, 1)
        this.tabs.splice(idx, 1)
        if (this.activeTabIdx === idx) {
            this.switchToTab(idx - 1)
            if (idx - 1 > 0) {
                this.router.navigate(['/apps/' + this.appType + '/' + this.appTab.app.id])
            } else {
                this.router.navigate(['/apps/' + this.appType + '/all'])
            }
        } else if (this.activeTabIdx > idx) {
            this.activeTabIdx = this.activeTabIdx - 1
        }
        if (event) {
            event.preventDefault()
        }
    }

    _refreshAppState(app) {
        this.servicesApi.getApp(app.id).subscribe(
            (data) => {
                this.msgSrv.add({
                    severity: 'success',
                    summary: 'App refreshed',
                    detail: 'Refreshing succeeded.',
                })

                htmlizeExtVersion(data)
                setDaemonStatusErred(data)

                // refresh app in app list
                for (const s of this.apps) {
                    if (s.id === data.id) {
                        Object.assign(s, data)
                        break
                    }
                }
                // refresh machine in opened tab if present
                for (const s of this.openedApps) {
                    if (s.app.id === data.id) {
                        Object.assign(s.app, data)
                        // Notify the child component about the update.
                        this.refreshedAppTab.next(this.appTab)
                        break
                    }
                }
            },
            (err) => {
                const msg = getErrorMessage(err)
                this.msgSrv.add({
                    severity: 'error',
                    summary: 'Error getting app state',
                    detail: 'Error getting state of app: ' + msg,
                    life: 10000,
                })
            }
        )
    }

    showAppMenu(event, appMenu, app) {
        appMenu.toggle(event)

        // connect method to refresh machine state
        this.appMenuItems[0].command = () => {
            this._refreshAppState(app)
        }
    }

    onRefreshApp(event) {
        this._refreshAppState(this.appTab.app)
    }

    refreshAppsList(appsTable) {
        appsTable.onLazyLoad.emit(appsTable.createLazyLoadMetadata())
    }

    /**
     * Modifies an active tab's label after renaming an app.
     *
     * This function is invoked when an app is renamed in a child
     * component, i.e. kea-app-tab or bind9-app-tab. As a result,
     * the label of the currently selected tab is changed to the
     * new app name.
     *
     * @param event holds new app name.
     */
    onRenameApp(event) {
        if (this.activeTabIdx > 0) {
            this.tabs[this.activeTabIdx].label = event
        }
    }
}
