import { Component, EventEmitter, Input, Output } from '@angular/core'
import { ConfigChecker, ConfigCheckerPreference } from '../backend'

/**
 * Presentational component to display the config checker metadata
 * and update the checker preferences.
 * It uses the table to list the config checkers. It presents the two- or
 * tri-state checkbox to specify the checker state. The checker state
 * changes are immediately passed to the event emitter. The trigger and
 * selector list are presented as chips with the fancy icons. The presented
 * metadata are extended with the description.
 */
@Component({
    selector: 'app-config-checker-preference-picker',
    templateUrl: './config-checker-preference-picker.component.html',
    styleUrls: ['./config-checker-preference-picker.component.sass'],
})
export class ConfigCheckerPreferencePickerComponent {
    /**
     * List of the config checkers.
     */
    @Input() checkers: ConfigChecker[] = null

    /**
     * Stream of the changed config checker preferences.
     */
    @Output() changePreferences = new EventEmitter<ConfigCheckerPreference[]>()

    /**
     * Use tri-state checkboxes to specify the checker state
     */
    @Input() allowInheritState: boolean = false

    /**
     * If true, displays only the checker name and state.
     */
    @Input() minimal: boolean = false

    /**
     * Loading state.
     */
    private _loading: boolean = true

    /**
     * Sets the loading state. The false value resets the changes.
     */
    @Input() set loading(isLoading: boolean) {
        this._loading = isLoading
        if (!isLoading) {
            this.changes = {}
        }
    }

    /**
     * If it's true, the data aren't ready yet.
     */
    get loading(): boolean {
        return this._loading
    }

    /**
     * List of provided checker state changes.
     */
    private changes: Record<string, ConfigChecker.StateEnum> = {}

    /**
     * It cycles the checker states. The order is enabled - disabled - inherit.
     * It skips the inherit state if the component is configured to disallow it.
     * @param state Checker state
     * @returns Next checker state
     */
    private _getNextState(state: ConfigChecker.StateEnum): ConfigChecker.StateEnum {
        if (state === ConfigChecker.StateEnum.Inherit) {
            return ConfigChecker.StateEnum.Enabled
        } else if (state === ConfigChecker.StateEnum.Enabled) {
            return ConfigChecker.StateEnum.Disabled
        } else {
            if (this.allowInheritState) {
                return ConfigChecker.StateEnum.Inherit
            } else {
                return ConfigChecker.StateEnum.Enabled
            }
        }
    }

    /**
     * Returns a fancy icon for the checker trigger. If the trigger is unknown
     * then returns no icon.
     * @param trigger The checker trigger
     * @returns Icon CSS classes
     */
    getTriggerIcon(trigger: string): string {
        switch (trigger) {
            case 'internal':
                return 'fa fa-eye-slash'
            case 'manual':
                return 'fa fa-hand-paper'
            case 'config change':
                return 'fa fa-tools'
            case 'host reservations change':
                return 'fa fa-registered'
            default:
                return null
        }
    }

    /**
     * Returns a fancy icon for the checker selector. If the selector is unknown
     * then returns no icon.
     * We don't have specialized icons for our daemons, and FontAwesome doesn't
     * contain any icons related to DHCP or DNS. But the chips with icons look
     * better than those without. The dices aren't the first thing you associate
     * with the Stork-supported daemons, but it has a little sense:
     * - Kea DHCPv4 is a die with a single 4-dots-side visible
     * - Kea DHCPv6 is a die with a single 6-dots-side visible
     * - Kea D2 daemon is a die with a single 2-dots-side visible
     * - General DHCP daemon is two dice with a single side visible
     * - General Kea daemon is a 6-side die in isometric projection
     * - Kea Control daemon is a 6-side die in isometric projection with
     *   highlighted one side
     * - General daemon is a fancy representation of 20-side die
     * - Bind 9 daemon is a circle with a single dot in the center because it
     *   has a dot similar to Kea DHCP dice (Bind 9 is a specific daemon), but
     *   the circle is opposite of a square (DNS is an entirely different thing
     *   than DHCP). Additionally, the circle is similar to the 20-side dice in
     *   the same way as the square. (20-side dice is a generalization of daemon).
     * @param selector The checker selector
     * @returns Icon CSS classes
     */
    getSelectorIcon(selector: string): string {
        switch (selector) {
            case 'each-daemon':
                return 'fa fa-dice-d20'
            case 'kea-daemon':
                return 'fa fa-dice-d6'
            case 'kea-ca-daemon':
                return 'fa fa-cube'
            case 'kea-dhcp-daemon':
                return 'fa fa-dice'
            case 'kea-dhcp-v4-daemon':
                return 'fa fa-dice-four'
            case 'kea-dhcp-v6-daemon':
                return 'fa fa-dice-six'
            case 'kea-d2-daemon':
                return 'fa fa-dice-two'
            case 'bind9-daemon':
                return 'fa fa-dot-circle'
            default:
                return null
        }
    }

    /**
     * Returns the description for a given checker. If the checker is unknown,
     * returns an empty string.
     * @param checkerName Configuration checker name
     * @returns Description of the checker
     */
    getCheckerDescription(checkerName: string): string {
        switch (checkerName) {
            case 'stat_cmds_presence':
                return 'The checker verifying if the stat_cmds hooks library is loaded.'
            case 'host_cmds_presence':
                return (
                    'The checker verifying if the host_cmds hooks library is ' + 'loaded when host backend is in use.'
                )
            case 'dispensable_shared_network':
                return (
                    'The checker verifying if a shared network can be removed ' +
                    'because it is empty or contains only one subnet.'
                )
            case 'dispensable_subnet':
                return (
                    'The checker verifying if a subnet can be removed because ' +
                    'it includes no pools and no reservations. The check is ' +
                    'skipped when the host_cmds hook library is loaded because ' +
                    'host reservations may be present in the database.'
                )
            case 'out_of_pool_reservation':
                return (
                    'The checker suggesting the use of out-of-pool host ' +
                    'reservation mode when there are subnets with all host ' +
                    'reservations outside of the dynamic pools.'
                )
            case 'overlapping_subnet':
                return 'The checker verifying if subnet prefixes do not overlap.'
            case 'canonical_prefix':
                return 'The checker verifying if subnet prefixes are in the ' + 'canonical form.'
            default:
                return ''
        }
    }

    /**
     * Returns the actual checker state. If no changes were made, returns
     * an original state.
     * @param checker Configuration checker
     * @returns Checker state
     */
    getActualState(checker: ConfigChecker): ConfigChecker.StateEnum {
        return this.changes[checker.name] ?? checker.state
    }

    /**
     * Returns true if any significant change was made.
     */
    get hasChanges(): boolean {
        return Object.keys(this.changes).length !== 0
    }

    /**
     * Callback called on checker state change.
     * @param event Generic change input event
     * @param checker Affected checker
     */
    onCheckerStateChanged(checker: ConfigChecker) {
        const originalState = checker.state
        const currentState = this.getActualState(checker)
        const nextState = this._getNextState(currentState)

        if (nextState === originalState) {
            delete this.changes[checker.name]
        } else {
            this.changes[checker.name] = nextState
        }
    }

    /**
     * Callback called on checker state changes submission. It emits an Angular
     * event with changed checker preference and sets the loading state.
     */
    onSubmit() {
        this.loading = true
        this.changePreferences.emit(
            Object.keys(this.changes).map((k) => ({
                name: k,
                state: this.changes[k],
            }))
        )
    }

    /**
     * Callback called on checker state reset.
     */
    onReset() {
        this.changes = {}
    }
}
