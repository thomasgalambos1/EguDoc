import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DatePipe, SlicePipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import { CardModule } from 'primeng/card';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';
import { TableModule } from 'primeng/table';
import { SkeletonModule } from 'primeng/skeleton';
import { AuthService } from '../core/services/auth.service';
import { RegistraturaService } from '../core/services/registratura.service';
import { Document, DocumentStatus } from '../core/models/document.model';

@Component({
  selector: 'app-dashboard',
  imports: [
    DatePipe, SlicePipe, RouterLink,
    CardModule, ButtonModule, TagModule, TableModule, SkeletonModule
  ],
  template: `
    <div class="flex flex-col gap-6 p-6">
      <div class="flex items-center justify-between">
        <div class="flex flex-col gap-1">
          <h1 class="m-0 text-2xl font-bold">Bun venit, {{ auth.userInfo()?.given_name ?? auth.userInfo()?.email }}</h1>
          <span class="text-sm" style="color: var(--p-text-muted-color)">Sistem de Gestiune Documente</span>
        </div>
        <p-button label="Document Nou" icon="pi pi-plus" routerLink="/registratura/nou" />
      </div>

      <div class="grid gap-6" style="grid-template-columns: repeat(4, 1fr);">
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--p-text-muted-color)">Total Documente</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().total }}</span>
            }
          </div>
        </p-card>
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--p-text-muted-color)">În Lucru</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().inLucru }}</span>
            }
          </div>
        </p-card>
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--p-text-muted-color)">Așteaptă Aprobare</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().fluxAprobare }}</span>
            }
          </div>
        </p-card>
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--p-text-muted-color)">Finalizate Azi</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().finalizateAzi }}</span>
            }
          </div>
        </p-card>
      </div>

      <p-card header="Documente Recente">
        <p-table
          [value]="recentDocuments()"
          [loading]="loading()"
          [paginator]="false"
          styleClass="p-datatable-sm"
        >
          <ng-template #header>
            <tr>
              <th>Nr. Înregistrare</th>
              <th>Tip</th>
              <th>Obiect</th>
              <th>Status</th>
              <th>Data</th>
              <th></th>
            </tr>
          </ng-template>
          <ng-template #body let-doc>
            <tr>
              <td><code>{{ doc.nr_inregistrare }}</code></td>
              <td>{{ doc.tip }}</td>
              <td style="max-width: 300px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                {{ doc.obiect }}
              </td>
              <td>
                <p-tag
                  [value]="statusLabel(doc.status)"
                  [severity]="statusSeverity(doc.status)"
                />
              </td>
              <td>{{ doc.data_inregistrare | date:'dd.MM.yyyy' }}</td>
              <td>
                <p-button
                  icon="pi pi-eye"
                  severity="secondary"
                  [text]="true"
                  [rounded]="true"
                  [routerLink]="['/registratura', doc.id]"
                />
              </td>
            </tr>
          </ng-template>
          <ng-template #emptymessage>
            <tr>
              <td colspan="6">
                <div class="flex flex-col items-center gap-3 p-6">
                  <i class="pi pi-inbox text-4xl" style="color: var(--p-text-muted-color)"></i>
                  <span style="color: var(--p-text-muted-color)">Nu există documente recente</span>
                  <p-button label="Înregistrează primul document" routerLink="/registratura/nou" />
                </div>
              </td>
            </tr>
          </ng-template>
        </p-table>
      </p-card>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DashboardComponent implements OnInit {
  auth = inject(AuthService);
  private registraturaService = inject(RegistraturaService);

  loading = signal(true);
  recentDocuments = signal<Document[]>([]);
  stats = signal({ total: 0, inLucru: 0, fluxAprobare: 0, finalizateAzi: 0 });

  ngOnInit(): void {
    this.loadDashboard();
  }

  private async loadDashboard(): Promise<void> {
    try {
      const result = await this.registraturaService.getDocuments({ limit: 10, page: 1 });
      this.recentDocuments.set(result.data ?? []);
      this.stats.set({
        total: result.total,
        inLucru: result.data.filter(d => d.status === 'IN_LUCRU').length,
        fluxAprobare: result.data.filter(d => d.status === 'FLUX_APROBARE').length,
        finalizateAzi: result.data.filter(d => d.status === 'FINALIZAT').length,
      });
    } catch {
      // Service unavailable during development — leave stats at zero
    } finally {
      this.loading.set(false);
    }
  }

  statusLabel(status: DocumentStatus): string {
    const labels: Record<DocumentStatus, string> = {
      INREGISTRAT: 'Înregistrat',
      ALOCAT_COMPARTIMENT: 'Alocat',
      IN_LUCRU: 'În Lucru',
      FLUX_APROBARE: 'Aprobare',
      FINALIZAT: 'Finalizat',
      ARHIVAT: 'Arhivat',
      ANULAT: 'Anulat',
    };
    return labels[status] ?? status;
  }

  statusSeverity(status: DocumentStatus): 'success' | 'info' | 'warn' | 'danger' | 'secondary' {
    const map: Record<DocumentStatus, 'success' | 'info' | 'warn' | 'danger' | 'secondary'> = {
      INREGISTRAT: 'info',
      ALOCAT_COMPARTIMENT: 'warn',
      IN_LUCRU: 'warn',
      FLUX_APROBARE: 'warn',
      FINALIZAT: 'success',
      ARHIVAT: 'secondary',
      ANULAT: 'danger',
    };
    return map[status] ?? 'secondary';
  }
}
