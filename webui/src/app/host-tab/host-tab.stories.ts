import { moduleMetadata, Meta, Story } from '@storybook/angular'
import mockAddon from 'storybook-addon-mock'
import { HttpClientModule } from '@angular/common/http'
import { FormsModule } from '@angular/forms'
import { NoopAnimationsModule } from '@angular/platform-browser/animations'
import { RouterModule } from '@angular/router'
import { RouterTestingModule } from '@angular/router/testing'
import { ConfirmationService, MessageService } from 'primeng/api'
import { ChipModule } from 'primeng/chip'
import { ConfirmDialogModule } from 'primeng/confirmdialog'
import { FieldsetModule } from 'primeng/fieldset'
import { OverlayPanelModule } from 'primeng/overlaypanel'
import { ProgressSpinnerModule } from 'primeng/progressspinner'
import { TableModule } from 'primeng/table'
import { TagModule } from 'primeng/tag'
import { ToastModule } from 'primeng/toast'
import { ToggleButtonModule } from 'primeng/togglebutton'
import { TreeModule } from 'primeng/tree'
import { DhcpClientClassSetViewComponent } from '../dhcp-client-class-set-view/dhcp-client-class-set-view.component'
import { DhcpOptionSetViewComponent } from '../dhcp-option-set-view/dhcp-option-set-view.component'
import { HelpTipComponent } from '../help-tip/help-tip.component'
import { HostTabComponent } from '../host-tab/host-tab.component'
import { IdentifierComponent } from '../identifier/identifier.component'
import { toastDecorator } from '../utils.stories'

export default {
    title: 'App/HostTab',
    component: HostTabComponent,
    decorators: [
        moduleMetadata({
            imports: [
                ChipModule,
                ConfirmDialogModule,
                FieldsetModule,
                FormsModule,
                HttpClientModule,
                NoopAnimationsModule,
                OverlayPanelModule,
                ProgressSpinnerModule,
                TableModule,
                RouterModule,
                RouterTestingModule,
                ToastModule,
                ToggleButtonModule,
                TreeModule,
                TagModule,
            ],
            declarations: [
                IdentifierComponent,
                DhcpClientClassSetViewComponent,
                DhcpOptionSetViewComponent,
                HelpTipComponent,
            ],
            providers: [ConfirmationService, MessageService],
        }),
        mockAddon,
        toastDecorator,
    ],
    parameters: {
        mockData: [
            {
                url: 'http://localhost/api/leases?hostId=1',
                method: 'GET',
                status: 200,
                delay: 2000,
                response: {
                    items: [],
                    conflicts: 0,
                    erredApps: 0,
                },
            },
        ],
    },
} as Meta

const Template: Story<HostTabComponent> = (args: HostTabComponent) => ({
    props: args,
})

export const ViewHost = Template.bind({})
ViewHost.args = {
    host: {
        id: 1,
        hostIdentifiers: [
            {
                idType: 'duid',
                idHexValue: '51:52:53:54',
            },
            {
                idType: 'hw-address',
                idHexValue: '51:52:53:54:55:56',
            },
        ],
        addressReservations: [
            {
                address: '2001:db8:1::1',
            },
            {
                address: '2001:db8:1::2',
            },
        ],
        prefixReservations: [
            {
                address: '2001:db8:2::/64',
            },
            {
                address: '2001:db8:3::/64',
            },
        ],
        hostname: 'mouse.example.org',
        subnetId: 1,
        subnetPrefix: '2001:db8:1::/64',
        localHosts: [
            {
                appId: 1,
                appName: 'frog',
                dataSource: 'config',
                clientClasses: ['access-point', 'router', 'cable-modem'],
            },
            {
                appId: 2,
                appName: 'mouse',
                dataSource: 'api',
                clientClasses: ['access-point', 'router', 'cable-modem'],
            },
        ],
    },
}
