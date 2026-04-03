import { Component, ChangeDetectionStrategy } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';

@Component({
  selector: 'app-document-detail',
  imports: [RouterLink, ButtonModule, CardModule],
  template: `
    <div class="flex flex-col gap-4 p-6">
      <div class="flex items-center gap-3">
        <p-button icon="pi pi-arrow-left" severity="secondary" [text]="true" routerLink="/registratura" />
        <h1 class="m-0 text-2xl font-bold">Detalii Document</h1>
      </div>
      <p-card>
        <p style="color: var(--p-text-muted-color)">Vizualizarea detaliată a documentului va fi implementată în faza următoare.</p>
      </p-card>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DocumentDetailComponent {}
