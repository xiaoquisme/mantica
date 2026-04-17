export interface Swimlane {
  id: string;
  workspace_id: string;
  name: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSwimlaneRequest {
  name: string;
}

export interface UpdateSwimlaneRequest {
  name?: string;
}

export interface ListSwimlanesResponse {
  swimlanes: Swimlane[];
  total: number;
}
