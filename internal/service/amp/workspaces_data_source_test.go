package amp_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/service/prometheusservice"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
)

func TestAccAMPWorkspacesDataSource_basic(t *testing.T) {
	rCount := strconv.Itoa(sdkacctest.RandIntRange(1, 4))
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.aws_prometheus_workspaces.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(t); acctest.PreCheckPartitionHasService(prometheusservice.EndpointsID, t) },
		ErrorCheck:                acctest.ErrorCheck(t, prometheusservice.EndpointsID),
		PreventPostDestroyRefresh: true,
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspacesDataSourceConfig_resources(rCount, rName),
			},
			{
				Config: testAccWorkspacesDataSourceConfig_all(rCount, rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkspacesExistsDataSource(dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "aliases.#", rCount),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", rCount),
					resource.TestCheckResourceAttr(dataSourceName, "workspace_ids.#", rCount),
				),
			},
		},
	})
}

func TestAccAMPWorkspacesDataSource_aliasPrefix(t *testing.T) {
	rCount := strconv.Itoa(sdkacctest.RandIntRange(1, 4))
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.aws_prometheus_workspaces.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                  func() { acctest.PreCheck(t); acctest.PreCheckPartitionHasService(prometheusservice.EndpointsID, t) },
		ErrorCheck:                acctest.ErrorCheck(t, prometheusservice.EndpointsID),
		PreventPostDestroyRefresh: true,
		ProtoV5ProviderFactories:  acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspacesDataSourceConfig_aliasPrefix(rCount, rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkspacesExistsDataSource(dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "aliases.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "workspace_ids.#", "1"),
				),
			},
		},
	})
}

func testAccWorkspacesDataSourceConfig_resources(rCount, rName string) string {
	return fmt.Sprintf(`
resource "aws_prometheus_workspace" "test" {
  count = %[1]s
  alias = "%[2]s-${count.index}"
}
`, rCount, rName)
}

func testAccWorkspacesDataSourceConfig_all(rCount, rName string) string {
	return fmt.Sprintf(`
%s
data "aws_prometheus_workspaces" "test" {
}
`, testAccWorkspacesDataSourceConfig_resources(rCount, rName))
}

func testAccWorkspacesDataSourceConfig_aliasPrefix(rCount, rName string) string {
	return fmt.Sprintf(`
%s
data "aws_prometheus_workspaces" "test" {
  alias_prefix = aws_prometheus_workspace.test[0].alias
}
`, testAccWorkspacesDataSourceConfig_resources(rCount, rName))
}

func testAccCheckWorkspacesExistsDataSource(addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("Can't find AMP workspaces data source: %s", addr)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("AMP workspaces data source ID not set")
		}

		return nil
	}
}
