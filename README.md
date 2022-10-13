# mirrosa

* pkg/rosa
    * pkg/rosa/sts - Specific code for validating STS rosa clusters
    * pkg/rosa/byovpc - Specific code for validating BYOVPC clusters
    * pkg/rosa - Shared AWS code for all ROSA clusters

* main.go
    * Input: ClusterID
    * Figure out what type of ROSA cluster that clusterID is
    * Output: Validate and print any differences in AWS

```mermaid
graph TD;
  C[OCM ClusterId]-->P[PrivateLink]
  P-->VPCES[VPC Endpoint Service]
  C-->STS
  C-->D[DNS Basedomain]
  C-->BYOVPC
  BYOVPC-->Subnet
  D-->R[Route53 Private Hosted Zone]
  D-->RPub[Route53 Public Hosted Zone]
  R-->APILB[api LB]
  R-->ALB[*.apps LB]
  R-->V[VPC ID]
  V-->Subnet[SubnetIDs]
  Subnet-->RT[Route Tables]
  V-->SG[Security Groups]
  V-->VPCE[S3 VPC Endpoint]
  V-->IGW[Internet Gateway]
  RT-->NAT[NAT Gateway]
```