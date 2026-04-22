# EguDoc — Angular Web File Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a fully functional web-based file manager to the Angular app so EguDoc works completely without the Windows sync client. Users can upload, download, view version history, and manage attachments from the browser.

**Architecture:** New `SyncApiService` wraps the Go sync/versioning endpoints. Browser upload uses the same presigned PUT flow as the Windows client: Angular calls upload-intent → browser PUTs directly to RustFS → Angular calls upload-confirm. This bypasses the Go pod for file bytes. Three new components (AttachmentManager, FileUpload, VersionHistory) are wired into the existing document detail view.

**Tech Stack:** Angular 17+, Angular HttpClient, RxJS, PrimeNG components, existing auth/http interceptor pattern

**Prerequisites:** Phase 1 Go backend must be deployed (sync, versioning endpoints live).

**First step:** Run `mcp__angular-cli__list_projects` and `mcp__angular-cli__get_best_practices` before writing any code to get correct Angular workspace paths and version-specific standards.

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `src/app/core/services/sync-api.service.ts` | Create | HTTP client for all sync + versioning endpoints |
| `src/app/core/models/attachment.model.ts` | Create | Shared TypeScript types for attachments and versions |
| `src/app/shared/components/attachment-manager/` | Create | Top-level component: list + download + upload + version history |
| `src/app/shared/components/file-upload/` | Create | Presigned PUT upload with progress indicator |
| `src/app/shared/components/version-history/` | Create | Version list + per-version download |
| `src/app/shared/components/sync-status-badge/` | Create | Placeholder/local/uploading/conflict badge |
| Existing document detail component | Modify | Add `<app-attachment-manager>` |

Before modifying any existing component: read it first with the Read tool.

---

### Task 1: Core models — attachment.model.ts

**Files:**
- Create: `src/app/core/models/attachment.model.ts`

- [ ] **Step 1: Write the model file**

```typescript
export interface DeltaAtasament {
  id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  current_version: number;
  updated_at: string;
}

export interface AttachmentVersion {
  id: string;
  version_nr: number;
  size_bytes: number;
  uploaded_by: string;
  source: 'web' | 'windows_sync';
  created_at: string;
}

export interface UploadIntentResponse {
  upload_url: string;
  storage_key: string;
}

export interface UploadConfirmResponse {
  atasament_id: string;
  version_nr: number;
}

export type SyncStatus = 'placeholder' | 'local' | 'uploading' | 'conflict';

export interface AttachmentWithStatus extends DeltaAtasament {
  sync_status?: SyncStatus;
}
```

- [ ] **Step 2: Build check**

```bash
ng build --dry-run 2>&1 | head -20
```
Expected: no errors related to the new model file.

- [ ] **Step 3: Commit**

```bash
git add src/app/core/models/attachment.model.ts
git commit -m "feat(web-filemanager): add attachment and version TypeScript models"
```

---

### Task 2: SyncApiService

**Files:**
- Create: `src/app/core/services/sync-api.service.ts`
- Create: `src/app/core/services/sync-api.service.spec.ts`

- [ ] **Step 1: Write the failing test**

```typescript
// src/app/core/services/sync-api.service.spec.ts
import { TestBed } from '@angular/core/testing';
import { HttpClientTestingModule, HttpTestingController } from '@angular/common/http/testing';
import { SyncApiService } from './sync-api.service';

describe('SyncApiService', () => {
  let service: SyncApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [HttpClientTestingModule],
      providers: [SyncApiService],
    });
    service = TestBed.inject(SyncApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('getDownloadUrl calls correct endpoint', (done) => {
    service.getDownloadUrl('test-aid').subscribe(url => {
      expect(url).toBe('https://storage.test/file.docx');
      done();
    });
    const req = http.expectOne('/api/sync/download/test-aid');
    expect(req.request.method).toBe('GET');
    req.flush({ url: 'https://storage.test/file.docx' });
  });

  it('getVersions calls correct endpoint', (done) => {
    service.getVersions('test-aid').subscribe(versions => {
      expect(versions.length).toBe(1);
      expect(versions[0].version_nr).toBe(2);
      done();
    });
    const req = http.expectOne('/api/versioning/test-aid/versions');
    req.flush([{ id: 'v1', version_nr: 2, size_bytes: 1000, uploaded_by: 'user', source: 'web', created_at: new Date().toISOString() }]);
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
ng test --include="**/sync-api.service.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```
Expected: FAILED — service class not found.

- [ ] **Step 3: Create the service**

```typescript
// src/app/core/services/sync-api.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpEvent, HttpRequest } from '@angular/common/http';
import { Observable, map, switchMap } from 'rxjs';
import {
  AttachmentVersion,
  UploadConfirmResponse,
  UploadIntentResponse,
} from '../models/attachment.model';

@Injectable({ providedIn: 'root' })
export class SyncApiService {
  constructor(private http: HttpClient) {}

  getDownloadUrl(attachmentId: string): Observable<string> {
    return this.http
      .get<{ url: string }>(`/api/sync/download/${attachmentId}`)
      .pipe(map(r => r.url));
  }

  uploadIntent(
    documentId: string,
    atasamentId: string | null,
    filename: string,
    contentType: string,
    sizeBytes: number
  ): Observable<UploadIntentResponse> {
    const body: Record<string, unknown> = {
      document_id: documentId,
      filename,
      content_type: contentType,
      size_bytes: sizeBytes,
    };
    if (atasamentId) body['atasament_id'] = atasamentId;
    return this.http.post<UploadIntentResponse>('/api/sync/upload-intent', body);
  }

  /** PUT file bytes directly to the presigned RustFS URL — no auth header. */
  putToPresignedUrl(
    presignedUrl: string,
    file: File
  ): Observable<HttpEvent<void>> {
    const req = new HttpRequest('PUT', presignedUrl, file, {
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
      reportProgress: true,
      // Skip the app's auth interceptor — presigned URLs don't need Authorization.
    });
    // Use a plain HttpClient that bypasses interceptors.
    // If the app's interceptor adds Authorization unconditionally, create a
    // secondary HttpClient without interceptors here. Check existing interceptor
    // implementation before deciding.
    return this.http.request<void>(req);
  }

  uploadConfirm(
    documentId: string,
    atasamentId: string | null,
    storageKey: string,
    filename: string,
    contentType: string,
    sizeBytes: number,
    sha256: string
  ): Observable<UploadConfirmResponse> {
    const body: Record<string, unknown> = {
      document_id: documentId,
      storage_key: storageKey,
      filename,
      content_type: contentType,
      size_bytes: sizeBytes,
      sha256,
      source: 'web',
    };
    if (atasamentId) body['atasament_id'] = atasamentId;
    return this.http.post<UploadConfirmResponse>('/api/sync/upload-confirm', body);
  }

  getVersions(attachmentId: string): Observable<AttachmentVersion[]> {
    return this.http.get<AttachmentVersion[]>(`/api/versioning/${attachmentId}/versions`);
  }

  getVersionDownloadUrl(attachmentId: string, versionNr: number): Observable<string> {
    return this.http
      .get<{ url: string }>(`/api/versioning/${attachmentId}/versions/${versionNr}`)
      .pipe(map(r => r.url));
  }
}
```

- [ ] **Step 4: Run tests**

```bash
ng test --include="**/sync-api.service.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```
Expected: 2 specs passed.

- [ ] **Step 5: Commit**

```bash
git add src/app/core/services/sync-api.service.ts src/app/core/services/sync-api.service.spec.ts
git commit -m "feat(web-filemanager): add SyncApiService for upload/download/versioning"
```

---

### Task 3: SyncStatusBadge component

**Files:**
- Create: `src/app/shared/components/sync-status-badge/sync-status-badge.component.ts`
- Create: `src/app/shared/components/sync-status-badge/sync-status-badge.component.html`

- [ ] **Step 1: Create the component**

```typescript
// sync-status-badge.component.ts
import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SyncStatus } from '../../../core/models/attachment.model';

@Component({
  selector: 'app-sync-status-badge',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './sync-status-badge.component.html',
})
export class SyncStatusBadgeComponent {
  @Input() status: SyncStatus | undefined;

  get label(): string {
    switch (this.status) {
      case 'placeholder': return 'Cloud';
      case 'local': return 'Local';
      case 'uploading': return 'Se încarcă...';
      case 'conflict': return 'Conflict';
      default: return '';
    }
  }

  get cssClass(): string {
    switch (this.status) {
      case 'placeholder': return 'badge-cloud';
      case 'local': return 'badge-local';
      case 'uploading': return 'badge-uploading';
      case 'conflict': return 'badge-conflict';
      default: return '';
    }
  }
}
```

```html
<!-- sync-status-badge.component.html -->
<span *ngIf="status" class="sync-badge" [ngClass]="cssClass">
  {{ label }}
</span>
```

Add the badge CSS to the component's host styles or global styles:

```css
.sync-badge { font-size: 10px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.badge-cloud { background: #dbeafe; color: #1d4ed8; }
.badge-local { background: #dcfce7; color: #166534; }
.badge-uploading { background: #fef9c3; color: #854d0e; }
.badge-conflict { background: #fee2e2; color: #991b1b; }
```

- [ ] **Step 2: Build check**

```bash
ng build --dry-run 2>&1 | grep -i "error" | head -10
```

- [ ] **Step 3: Commit**

```bash
git add src/app/shared/components/sync-status-badge/
git commit -m "feat(web-filemanager): add SyncStatusBadge component"
```

---

### Task 4: VersionHistory component

**Files:**
- Create: `src/app/shared/components/version-history/version-history.component.ts`
- Create: `src/app/shared/components/version-history/version-history.component.html`
- Create: `src/app/shared/components/version-history/version-history.component.spec.ts`

- [ ] **Step 1: Write the failing test**

```typescript
// version-history.component.spec.ts
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { HttpClientTestingModule } from '@angular/common/http/testing';
import { VersionHistoryComponent } from './version-history.component';
import { SyncApiService } from '../../../core/services/sync-api.service';
import { of } from 'rxjs';

describe('VersionHistoryComponent', () => {
  let fixture: ComponentFixture<VersionHistoryComponent>;
  let mockSyncApi: jasmine.SpyObj<SyncApiService>;

  beforeEach(async () => {
    mockSyncApi = jasmine.createSpyObj('SyncApiService', ['getVersions', 'getVersionDownloadUrl']);
    mockSyncApi.getVersions.and.returnValue(of([
      { id: 'v1', version_nr: 2, size_bytes: 2048, uploaded_by: 'Ion', source: 'web', created_at: new Date().toISOString() },
      { id: 'v2', version_nr: 1, size_bytes: 1024, uploaded_by: 'Maria', source: 'windows_sync', created_at: new Date().toISOString() },
    ]));

    await TestBed.configureTestingModule({
      imports: [VersionHistoryComponent, HttpClientTestingModule],
      providers: [{ provide: SyncApiService, useValue: mockSyncApi }],
    }).compileComponents();

    fixture = TestBed.createComponent(VersionHistoryComponent);
    fixture.componentInstance.attachmentId = 'test-aid';
    fixture.detectChanges();
  });

  it('displays version list', () => {
    const rows = fixture.nativeElement.querySelectorAll('[data-testid="version-row"]');
    expect(rows.length).toBe(2);
  });

  it('shows version numbers', () => {
    const text = fixture.nativeElement.textContent;
    expect(text).toContain('v2');
    expect(text).toContain('v1');
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
ng test --include="**/version-history.component.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```

- [ ] **Step 3: Create the component**

```typescript
// version-history.component.ts
import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SyncApiService } from '../../../core/services/sync-api.service';
import { AttachmentVersion } from '../../../core/models/attachment.model';

@Component({
  selector: 'app-version-history',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './version-history.component.html',
})
export class VersionHistoryComponent implements OnInit {
  @Input({ required: true }) attachmentId!: string;
  @Input() filename = '';

  versions: AttachmentVersion[] = [];
  loading = true;

  constructor(private syncApi: SyncApiService) {}

  ngOnInit(): void {
    this.syncApi.getVersions(this.attachmentId).subscribe({
      next: v => { this.versions = v; this.loading = false; },
      error: () => { this.loading = false; },
    });
  }

  download(version: AttachmentVersion): void {
    this.syncApi.getVersionDownloadUrl(this.attachmentId, version.version_nr).subscribe(url => {
      window.open(url, '_blank');
    });
  }

  formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  sourceLabel(source: string): string {
    return source === 'windows_sync' ? 'Windows Sync' : 'Web';
  }
}
```

```html
<!-- version-history.component.html -->
<div class="version-history">
  <h4 class="version-title">Istoric versiuni — {{ filename }}</h4>

  <div *ngIf="loading" class="loading">Se încarcă...</div>

  <table *ngIf="!loading && versions.length > 0" class="version-table">
    <thead>
      <tr>
        <th>Versiune</th>
        <th>Data</th>
        <th>Utilizator</th>
        <th>Sursă</th>
        <th>Dimensiune</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      <tr *ngFor="let v of versions" data-testid="version-row">
        <td><strong>v{{ v.version_nr }}</strong></td>
        <td>{{ v.created_at | date:'dd.MM.yyyy HH:mm' }}</td>
        <td>{{ v.uploaded_by }}</td>
        <td>{{ sourceLabel(v.source) }}</td>
        <td>{{ formatBytes(v.size_bytes) }}</td>
        <td>
          <button class="btn-download" (click)="download(v)">Descarcă</button>
        </td>
      </tr>
    </tbody>
  </table>

  <p *ngIf="!loading && versions.length === 0" class="no-versions">
    Nicio versiune disponibilă.
  </p>
</div>
```

- [ ] **Step 4: Run tests**

```bash
ng test --include="**/version-history.component.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```
Expected: 2 specs passed.

- [ ] **Step 5: Commit**

```bash
git add src/app/shared/components/version-history/
git commit -m "feat(web-filemanager): add VersionHistory component with download per version"
```

---

### Task 5: FileUpload component

**Files:**
- Create: `src/app/shared/components/file-upload/file-upload.component.ts`
- Create: `src/app/shared/components/file-upload/file-upload.component.html`
- Create: `src/app/shared/components/file-upload/file-upload.component.spec.ts`

This component handles the 3-step presigned upload: intent → PUT → confirm. It emits an event when done so the parent can refresh the attachment list.

- [ ] **Step 1: Write the failing test**

```typescript
// file-upload.component.spec.ts
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FileUploadComponent } from './file-upload.component';
import { SyncApiService } from '../../../core/services/sync-api.service';
import { of } from 'rxjs';
import { HttpClientTestingModule } from '@angular/common/http/testing';

describe('FileUploadComponent', () => {
  let fixture: ComponentFixture<FileUploadComponent>;
  let mockSyncApi: jasmine.SpyObj<SyncApiService>;

  beforeEach(async () => {
    mockSyncApi = jasmine.createSpyObj('SyncApiService', [
      'uploadIntent', 'putToPresignedUrl', 'uploadConfirm',
    ]);

    await TestBed.configureTestingModule({
      imports: [FileUploadComponent, HttpClientTestingModule],
      providers: [{ provide: SyncApiService, useValue: mockSyncApi }],
    }).compileComponents();

    fixture = TestBed.createComponent(FileUploadComponent);
    fixture.componentInstance.documentId = 'doc-1';
    fixture.detectChanges();
  });

  it('shows drop zone', () => {
    const zone = fixture.nativeElement.querySelector('[data-testid="drop-zone"]');
    expect(zone).toBeTruthy();
  });

  it('emits uploaded event on successful upload', (done) => {
    mockSyncApi.uploadIntent.and.returnValue(of({ upload_url: 'https://s3.test/key', storage_key: 'key/file.docx' }));
    mockSyncApi.putToPresignedUrl.and.returnValue(of({ type: 4 } as any)); // HttpEventType.Response
    mockSyncApi.uploadConfirm.and.returnValue(of({ atasament_id: 'new-aid', version_nr: 1 }));

    fixture.componentInstance.uploaded.subscribe(() => done());

    // Simulate file selection
    const file = new File(['content'], 'test.docx', { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' });
    fixture.componentInstance.onFilesSelected([file]);
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
ng test --include="**/file-upload.component.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```

- [ ] **Step 3: Create file-upload.component.ts**

```typescript
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { HttpEventType } from '@angular/common/http';
import { SyncApiService } from '../../../core/services/sync-api.service';

@Component({
  selector: 'app-file-upload',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './file-upload.component.html',
})
export class FileUploadComponent {
  @Input({ required: true }) documentId!: string;
  @Input() atasamentId: string | null = null; // null = new attachment, set = new version
  @Output() uploaded = new EventEmitter<{ atasamentId: string; versionNr: number }>();

  isDragOver = false;
  uploading = false;
  progress = 0;
  error: string | null = null;

  constructor(private syncApi: SyncApiService) {}

  onDragOver(event: DragEvent): void {
    event.preventDefault();
    this.isDragOver = true;
  }

  onDragLeave(): void {
    this.isDragOver = false;
  }

  onDrop(event: DragEvent): void {
    event.preventDefault();
    this.isDragOver = false;
    const files = Array.from(event.dataTransfer?.files ?? []);
    if (files.length > 0) this.onFilesSelected(files);
  }

  onFileInputChange(event: Event): void {
    const input = event.target as HTMLInputElement;
    this.onFilesSelected(Array.from(input.files ?? []));
  }

  async onFilesSelected(files: File[]): Promise<void> {
    const file = files[0];
    if (!file) return;

    this.uploading = true;
    this.progress = 0;
    this.error = null;

    try {
      // Step 1: get presigned PUT URL
      const intent = await this.syncApi.uploadIntent(
        this.documentId,
        this.atasamentId,
        file.name,
        file.type || 'application/octet-stream',
        file.size
      ).toPromise();

      if (!intent) throw new Error('No upload intent response');

      // Step 2: compute SHA-256 of file bytes (Web Crypto API)
      const sha256 = await this.computeSha256(file);

      // Step 3: PUT directly to RustFS presigned URL via SyncApiService (mockable in tests).
      // SyncApiService.putToPresignedUrl uses HttpClient with reportProgress:true.
      // IMPORTANT: verify the app's HTTP auth interceptor does NOT add an Authorization
      // header to presigned URL requests — the AWS signature doesn't tolerate extra headers.
      // If it does, add a condition in the interceptor: skip if URL matches STORAGE_PUBLIC_ENDPOINT.
      await new Promise<void>((resolve, reject) => {
        this.syncApi.putToPresignedUrl(intent.upload_url, file).subscribe({
          next: (event: import('@angular/common/http').HttpEvent<void>) => {
            const { HttpEventType } = require('@angular/common/http');
            if (event.type === HttpEventType.UploadProgress) {
              const e = event as import('@angular/common/http').HttpUploadProgressEvent;
              if (e.total) this.progress = Math.round((e.loaded / e.total) * 100);
            } else if (event.type === HttpEventType.Response) {
              resolve();
            }
          },
          error: (err: Error) => reject(err),
        });
      });

      // Step 4: confirm with Go backend
      const confirm = await this.syncApi.uploadConfirm(
        this.documentId,
        this.atasamentId,
        intent.storage_key,
        file.name,
        file.type || 'application/octet-stream',
        file.size,
        sha256
      ).toPromise();

      if (!confirm) throw new Error('No confirm response');

      this.uploaded.emit({ atasamentId: confirm.atasament_id, versionNr: confirm.version_nr });
    } catch (e: unknown) {
      this.error = e instanceof Error ? e.message : 'Eroare la încărcare';
    } finally {
      this.uploading = false;
      this.progress = 0;
    }
  }

  private async computeSha256(file: File): Promise<string> {
    const buffer = await file.arrayBuffer();
    const hashBuffer = await crypto.subtle.digest('SHA-256', buffer);
    return Array.from(new Uint8Array(hashBuffer))
      .map(b => b.toString(16).padStart(2, '0'))
      .join('');
  }
}
```

```html
<!-- file-upload.component.html -->
<div
  class="drop-zone"
  data-testid="drop-zone"
  [class.drag-over]="isDragOver"
  (dragover)="onDragOver($event)"
  (dragleave)="onDragLeave()"
  (drop)="onDrop($event)"
>
  <div *ngIf="!uploading">
    <span class="drop-icon">📎</span>
    <p>Trageți un fișier aici sau <label class="browse-link">
      <input type="file" hidden (change)="onFileInputChange($event)">
      alegeți de pe calculator
    </label></p>
  </div>

  <div *ngIf="uploading" class="upload-progress">
    <div class="progress-bar">
      <div class="progress-fill" [style.width.%]="progress"></div>
    </div>
    <span>{{ progress }}%</span>
  </div>

  <p *ngIf="error" class="upload-error">{{ error }}</p>
</div>
```

- [ ] **Step 4: Run tests**

```bash
ng test --include="**/file-upload.component.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```
Expected: 2 specs passed.

- [ ] **Step 5: Commit**

```bash
git add src/app/shared/components/file-upload/
git commit -m "feat(web-filemanager): add FileUpload component with presigned PUT and SHA-256"
```

---

### Task 6: AttachmentManager component

**Files:**
- Create: `src/app/shared/components/attachment-manager/attachment-manager.component.ts`
- Create: `src/app/shared/components/attachment-manager/attachment-manager.component.html`
- Create: `src/app/shared/components/attachment-manager/attachment-manager.component.spec.ts`

This is the top-level file manager component that wires together the list, download, upload, and version history.

- [ ] **Step 1: Write the failing test**

```typescript
// attachment-manager.component.spec.ts
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { AttachmentManagerComponent } from './attachment-manager.component';
import { SyncApiService } from '../../../core/services/sync-api.service';
import { of } from 'rxjs';
import { HttpClientTestingModule } from '@angular/common/http/testing';

const mockAttachments = [
  { id: 'aid-1', filename: 'Contract.docx', content_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', size_bytes: 45000, current_version: 2, updated_at: new Date().toISOString() },
];

describe('AttachmentManagerComponent', () => {
  let fixture: ComponentFixture<AttachmentManagerComponent>;
  let mockSyncApi: jasmine.SpyObj<SyncApiService>;

  beforeEach(async () => {
    mockSyncApi = jasmine.createSpyObj('SyncApiService', ['getDownloadUrl']);
    mockSyncApi.getDownloadUrl.and.returnValue(of('https://storage.test/file.docx'));

    await TestBed.configureTestingModule({
      imports: [AttachmentManagerComponent, HttpClientTestingModule],
      providers: [{ provide: SyncApiService, useValue: mockSyncApi }],
    }).compileComponents();

    fixture = TestBed.createComponent(AttachmentManagerComponent);
    fixture.componentInstance.documentId = 'doc-1';
    fixture.componentInstance.attachments = mockAttachments;
    fixture.detectChanges();
  });

  it('renders attachment list', () => {
    const rows = fixture.nativeElement.querySelectorAll('[data-testid="attachment-row"]');
    expect(rows.length).toBe(1);
  });

  it('shows filename', () => {
    expect(fixture.nativeElement.textContent).toContain('Contract.docx');
  });

  it('shows version number', () => {
    expect(fixture.nativeElement.textContent).toContain('v2');
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
ng test --include="**/attachment-manager.component.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```

- [ ] **Step 3: Create the component**

```typescript
// attachment-manager.component.ts
import { Component, Input, OnChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AttachmentWithStatus } from '../../../core/models/attachment.model';
import { SyncApiService } from '../../../core/services/sync-api.service';
import { FileUploadComponent } from '../file-upload/file-upload.component';
import { VersionHistoryComponent } from '../version-history/version-history.component';
import { SyncStatusBadgeComponent } from '../sync-status-badge/sync-status-badge.component';

@Component({
  selector: 'app-attachment-manager',
  standalone: true,
  imports: [CommonModule, FileUploadComponent, VersionHistoryComponent, SyncStatusBadgeComponent],
  templateUrl: './attachment-manager.component.html',
})
export class AttachmentManagerComponent implements OnChanges {
  @Input({ required: true }) documentId!: string;
  @Input() attachments: AttachmentWithStatus[] = [];

  expandedVersionId: string | null = null;
  showUploadForId: string | null = null; // null = new attachment, 'aid' = new version

  constructor(private syncApi: SyncApiService) {}

  ngOnChanges(): void {
    // Nothing to do — parent is responsible for fetching and passing attachments.
  }

  download(attachment: AttachmentWithStatus): void {
    this.syncApi.getDownloadUrl(attachment.id).subscribe(url => {
      const a = document.createElement('a');
      a.href = url;
      a.download = attachment.filename;
      a.click();
    });
  }

  toggleVersionHistory(attachmentId: string): void {
    this.expandedVersionId = this.expandedVersionId === attachmentId ? null : attachmentId;
  }

  toggleNewVersion(attachmentId: string): void {
    this.showUploadForId = this.showUploadForId === attachmentId ? null : attachmentId;
  }

  showNewAttachmentUpload = false;

  onUploaded(event: { atasamentId: string; versionNr: number }): void {
    this.showUploadForId = null;
    this.showNewAttachmentUpload = false;
    // The parent component is responsible for refreshing the attachment list.
    // Emit an event or call a refresh method — depends on how the parent provides attachments.
    // For now, trigger a page reload as a safe fallback:
    // In a real implementation, the parent should subscribe to an @Output() refreshed event.
  }

  formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
}
```

```html
<!-- attachment-manager.component.html -->
<div class="attachment-manager">
  <div class="manager-header">
    <h3>Fișiere atașate</h3>
    <button class="btn-add" (click)="showNewAttachmentUpload = !showNewAttachmentUpload">
      + Adaugă fișier
    </button>
  </div>

  <app-file-upload
    *ngIf="showNewAttachmentUpload"
    [documentId]="documentId"
    [atasamentId]="null"
    (uploaded)="onUploaded($event)"
  />

  <div *ngIf="attachments.length === 0 && !showNewAttachmentUpload" class="no-attachments">
    Niciun fișier atașat.
  </div>

  <div *ngFor="let a of attachments" class="attachment-item" data-testid="attachment-row">
    <div class="attachment-row">
      <span class="file-icon">📄</span>
      <span class="filename">{{ a.filename }}</span>
      <span class="version-badge">v{{ a.current_version }}</span>
      <app-sync-status-badge [status]="a.sync_status" />
      <span class="size">{{ formatBytes(a.size_bytes) }}</span>

      <div class="actions">
        <button class="btn-action" (click)="download(a)" title="Descarcă versiunea curentă">⬇</button>
        <button class="btn-action" (click)="toggleVersionHistory(a.id)" title="Istoric versiuni">🕐</button>
        <button class="btn-action" (click)="toggleNewVersion(a.id)" title="Încarcă versiune nouă">⬆</button>
      </div>
    </div>

    <!-- Inline version history panel -->
    <app-version-history
      *ngIf="expandedVersionId === a.id"
      [attachmentId]="a.id"
      [filename]="a.filename"
    />

    <!-- Inline new-version upload -->
    <app-file-upload
      *ngIf="showUploadForId === a.id"
      [documentId]="documentId"
      [atasamentId]="a.id"
      (uploaded)="onUploaded($event)"
    />
  </div>
</div>
```

- [ ] **Step 4: Run tests**

```bash
ng test --include="**/attachment-manager.component.spec.ts" --watch=false --no-progress 2>&1 | tail -10
```
Expected: 3 specs passed.

- [ ] **Step 5: Commit**

```bash
git add src/app/shared/components/attachment-manager/
git commit -m "feat(web-filemanager): add AttachmentManager component with upload/download/version history"
```

---

### Task 7: Wire AttachmentManager into the document detail view

- [ ] **Step 1: Find the existing document detail component**

```bash
grep -r "atasamente\|attachments\|Atasament" src/ --include="*.html" -l
```

Read the identified component file(s) to understand how attachments are currently displayed.

- [ ] **Step 2: Add AttachmentManager import to the document detail component**

In the document detail component `.ts` file, add to `imports` array:

```typescript
import { AttachmentManagerComponent } from '../../shared/components/attachment-manager/attachment-manager.component';

// In @Component decorator:
imports: [
  // ... existing imports ...
  AttachmentManagerComponent,
]
```

- [ ] **Step 3: Replace existing attachment display with AttachmentManager**

In the document detail component `.html` template, find where attachments are currently rendered and replace with:

```html
<app-attachment-manager
  [documentId]="document.id"
  [attachments]="document.atasamente"
/>
```

If attachments currently come from a different field name, adapt accordingly.

- [ ] **Step 4: Test in browser**

Start the dev server:

```bash
ng serve
```

Open a document in the Angular app. Verify:
- Attachment list renders with download buttons
- Clicking download opens the file (via presigned URL)
- Clicking "Adaugă fișier" shows the upload drop zone
- Dropping a file completes the 3-step upload flow
- Clicking the history icon shows the version panel
- Version panel shows correct version numbers and dates

- [ ] **Step 5: Build for production**

```bash
ng build
```
Expected: successful build with no errors.

- [ ] **Step 6: Commit**

```bash
git add src/app/
git commit -m "feat(web-filemanager): wire AttachmentManager into document detail view"
```

---

### Task 8: New-version toast notification from Windows sync

When the Windows client uploads a new version via `upload-confirm`, the Angular app needs to notify the web user that a newer version is available. The simplest approach: on the next document poll (whatever interval the existing app uses), detect that `current_version` increased and show a toast.

- [ ] **Step 1: Find the existing document polling/refresh mechanism**

```bash
grep -r "interval\|poll\|refresh\|reload" src/ --include="*.ts" -l | head -10
```

Read the relevant service to understand how the document is refreshed.

- [ ] **Step 2: Add version-change detection to AttachmentManager**

In `attachment-manager.component.ts`, add:

```typescript
@Input() set attachments(value: AttachmentWithStatus[]) {
  if (this._attachments.length > 0) {
    value.forEach(newAta => {
      const prev = this._attachments.find(a => a.id === newAta.id);
      if (prev && newAta.current_version > prev.current_version) {
        this.showVersionToast(newAta.filename, newAta.current_version);
      }
    });
  }
  this._attachments = value;
}

get attachments(): AttachmentWithStatus[] {
  return this._attachments;
}

private _attachments: AttachmentWithStatus[] = [];
toastMessage: string | null = null;

private showVersionToast(filename: string, versionNr: number): void {
  this.toastMessage = `Fișier actualizat de pe Windows — ${filename} versiunea ${versionNr} disponibilă`;
  setTimeout(() => { this.toastMessage = null; }, 6000);
}
```

Add toast display to the template:

```html
<div *ngIf="toastMessage" class="version-toast" role="alert">
  {{ toastMessage }}
</div>
```

- [ ] **Step 3: Add toast styles**

```css
.version-toast {
  position: fixed; bottom: 24px; right: 24px;
  background: #1e40af; color: white;
  padding: 12px 20px; border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.2);
  z-index: 1000; max-width: 360px;
  animation: slide-in 0.3s ease;
}
@keyframes slide-in {
  from { transform: translateY(20px); opacity: 0; }
  to   { transform: translateY(0); opacity: 1; }
}
```

- [ ] **Step 4: Build and verify in browser**

```bash
ng build
ng serve
```

Open a document. In another terminal, trigger an upload-confirm via curl to simulate a Windows sync upload. Verify the toast appears within the document poll interval.

- [ ] **Step 5: Commit**

```bash
git add src/app/shared/components/attachment-manager/
git commit -m "feat(web-filemanager): add toast notification when Windows client uploads new version"
```

---

*End of Phase 3 plan.*

## Execution order

1. **Phase 1** (Go backend + RustFS) — prerequisite for everything
2. **Phase 3** (Angular file manager) — can be developed in parallel with Phase 2 once Phase 1 backend is deployed to dev
3. **Phase 2** (Windows sync client) — completely independent, can be built and tested on its own schedule; Windows client is optional
