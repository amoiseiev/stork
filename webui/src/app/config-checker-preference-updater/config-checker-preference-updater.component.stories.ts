import { HttpClientModule } from '@angular/common/http'
import { BrowserAnimationsModule, NoopAnimationsModule } from '@angular/platform-browser/animations'
import { Meta, moduleMetadata, Story } from '@storybook/angular'
import { MessageService } from 'primeng/api'
import { ChipModule } from 'primeng/chip'
import { OverlayPanelModule } from 'primeng/overlaypanel'
import { TableModule } from 'primeng/table'
import { ConfigChecker, ConfigCheckerPreferences, ConfigCheckers, ServicesService } from '../backend'
import { ConfigCheckerPreferencePickerComponent } from '../config-checker-preference-picker/config-checker-preference-picker.component'
import { HelpTipComponent } from '../help-tip/help-tip.component'
import { ConfigCheckerPreferenceUpdaterComponent } from './config-checker-preference-updater.component'
import mockAddon from 'storybook-addon-mock'
import { action } from '@storybook/addon-actions'
import { toastDecorator } from '../utils.stories'
import { ToastModule } from 'primeng/toast'
import { ButtonModule } from 'primeng/button'

const mockData: ConfigCheckers = {
    items: [
        {
            name: 'out_of_pool_reservation',
            selectors: ['each-daemon', 'kea-daemon'],
            state: ConfigChecker.StateEnum.Disabled,
            triggers: ['manual', 'config change'],
            globallyEnabled: false,
        },
        {
            name: 'dispensable_subnet',
            selectors: ['each-daemon'],
            state: ConfigChecker.StateEnum.Enabled,
            triggers: ['manual', 'config change'],
            globallyEnabled: true,
        },
    ],
    total: 2,
}

export default {
    title: 'App/ConfigCheckerPreferenceUpdater',
    component: ConfigCheckerPreferenceUpdaterComponent,
    decorators: [
        moduleMetadata({
            imports: [
                TableModule,
                ChipModule,
                OverlayPanelModule,
                BrowserAnimationsModule,
                HttpClientModule,
                ToastModule,
                ButtonModule,
            ],
            declarations: [
                HelpTipComponent,
                ConfigCheckerPreferenceUpdaterComponent,
                ConfigCheckerPreferencePickerComponent,
            ],
            providers: [MessageService, ServicesService],
        }),
        mockAddon,
        toastDecorator,
    ],
    argTypes: {
        daemonID: {
            type: { name: 'number', required: false },
        },
        minimal: {
            type: 'boolean',
            defaultValue: false,
        },
    },
    parameters: {
        mockData: [
            {
                url: 'http://localhost/api/daemons/:daemonId/config-checkers',
                method: 'GET',
                status: 200,
                response: mockData,
            },
            {
                url: 'http://localhost/api/daemons/:daemonId/config-checkers',
                method: 'PUT',
                status: 200,
                delay: 2000,
                response: (request) => {
                    const { body } = request
                    const preferences: ConfigCheckerPreferences = JSON.parse(body)
                    action('onUpdatePreferences')(preferences.items)

                    for (let preference of preferences.items) {
                        for (let checker of mockData.items) {
                            if (preference.name === checker.name) {
                                checker.state = preference.state
                            }
                        }
                    }
                    return mockData
                },
            },
        ],
    },
} as Meta

const Template: Story<ConfigCheckerPreferenceUpdaterComponent> = (args: ConfigCheckerPreferenceUpdaterComponent) => ({
    props: args,
})

export const GlobalCheckers = Template.bind({})
GlobalCheckers.args = {
    daemonID: null,
}

export const DaemonCheckers = Template.bind({})
DaemonCheckers.args = {
    daemonID: 1,
}
