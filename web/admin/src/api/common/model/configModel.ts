export interface DistrictModel {
  id: number;
  name: string;
  parent_id: number;
  has_children: boolean;
  children?: DistrictModel[];
}
