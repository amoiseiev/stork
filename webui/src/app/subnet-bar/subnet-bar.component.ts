import { Component, Input } from '@angular/core'
import { Subnet } from '../backend'

import { clamp, datetimeToLocal } from '../utils'

/**
 * Component that presents subnet as a bar with a sub-bar that shows utilizations in %.
 * It also shows details in a tooltip.
 */
@Component({
    selector: 'app-subnet-bar',
    templateUrl: './subnet-bar.component.html',
    styleUrls: ['./subnet-bar.component.sass'],
})
export class SubnetBarComponent {
    _subnet: Subnet
    style: any
    tooltip = ''

    constructor() {}

    @Input()
    set subnet(subnet: Subnet) {
        this._subnet = subnet

        const util: number = subnet.addrUtilization ? subnet.addrUtilization : 0

        const style = {
            // In some cases the utilization may be incorrect - less than
            // zero or greater than 100%. We need to truncate the value
            // to avoid a subnet bar overlapping other elements.
            width: clamp(util, 0, 100) + '%',
        }

        if (util > 100) {
            style['background-color'] = '#7C9FDE' // blue-ish
        } else if (util > 90) {
            style['background-color'] = '#faa' // red-ish
        } else if (util > 80) {
            style['background-color'] = '#ffcf76' // orange-ish
        } else {
            style['background-color'] = '#abffa8' // green-ish
        }

        this.style = style

        if (this._subnet.stats) {
            const stats = this._subnet.stats
            const lines = []
            if (util > 100) {
                lines.push('Warning! Utilization is greater than 100%. Data is unreliable.')
                lines.push(
                    'This problem is caused by a Kea limitation - addresses/NAs/PDs in out-of-pool host reservations are reported as assigned but excluded from the total counters.'
                )
                lines.push(
                    'Please manually check that the pool has free addresses and make sure that Kea and Stork are up-to-date.'
                )
                lines.push('')
            }
            if (this._subnet.subnet.includes('.')) {
                // DHCPv4 stats
                lines.push('Total: ' + stats['total-addresses'].toLocaleString('en-US'))
                lines.push('Assigned: ' + stats['assigned-addresses'].toLocaleString('en-US'))
                lines.push('Declined: ' + stats['declined-addresses'].toLocaleString('en-US'))
            } else {
                // DHCPv6 stats
                // IPv6 addresses
                if (stats['total-nas'] !== undefined) {
                    let total = stats['total-nas']
                    if (total === -1) {
                        total = Number.MAX_SAFE_INTEGER
                    }
                    lines.push('Total NAs: ' + total.toLocaleString('en-US'))
                }
                if (stats['assigned-nas'] !== undefined) {
                    lines.push('Assigned NAs: ' + stats['assigned-nas'].toLocaleString('en-US'))
                }
                if (stats['declined-nas'] !== undefined) {
                    lines.push('Declined NAs: ' + stats['declined-nas'].toLocaleString('en-US'))
                }
                // PDs
                if (stats['total-pds'] !== undefined) {
                    let total = stats['total-pds']
                    if (total === -1) {
                        total = Number.MAX_SAFE_INTEGER
                    }
                    lines.push('Total PDs: ' + total.toLocaleString('en-US'))
                }
                if (stats['assigned-pds'] !== undefined) {
                    lines.push('Assigned PDs: ' + stats['assigned-pds'].toLocaleString('en-US'))
                }
            }
            lines.push('Collected at: ' + datetimeToLocal(this._subnet.statsCollectedAt))
            this.tooltip = lines.join('<br>')
        } else {
            this.tooltip = 'No stats yet'
            style['background-color'] = '#ccc' // grey
        }
    }

    get subnet() {
        return this._subnet
    }
}
